package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/golang-jwt/jwt/v5"
)

// adminAudience — il valore del claim `aud` delle sessioni del pannello.
//
// Il segreto di firma è lo stesso delle app degli atleti, quindi senza questo
// claim un JWT da atleta sarebbe una firma valida anche qui: resterebbe in
// piedi solo perché il `sub` è l'id di un utente e la ricerca in `admins`
// fallirebbe. Cioè la separazione dipenderebbe dal fatto che due tabelle non
// condividano un UUID — vero, ma per caso. Con `aud` invece i due tipi di
// sessione non sono intercambiabili per costruzione, e la cosa non cambia se un
// giorno le chiavi cambiano forma.
const adminAudience = "elite-admin"

// adminSessionTTL — durata della sessione del pannello. Molto più corta dei 30
// giorni degli atleti: un telefono resta in tasca, un browser sulla scrivania
// no, e da qui si vedono i dati di tutti gli iscritti.
const adminSessionTTL = 12 * time.Hour

// ErrNotAdmin — nessun amministratore attivo per quell'indirizzo. Non arriva
// mai fino all'utente (vedi SendLoginLink), serve ai log e ai test.
var ErrNotAdmin = errors.New("not an admin")

// AdminSession — quello che si ricava da un cookie valido.
type AdminSession struct {
	AdminID string
	// CSRF è il segreto per-sessione che i form del pannello rimandano in un
	// campo nascosto. Sta dentro il JWT invece che in una tabella: è già
	// firmato, scade con la sessione e cambia a ogni accesso, quindi non c'è
	// niente da conservare né da ripulire.
	CSRF string
}

type AdminService struct {
	admins    *repository.AdminRepo
	jwtSecret []byte
	isDev     bool
	mailer    Mailer
	// baseURL è l'origine da cui il pannello è raggiungibile — quella del
	// backend, non del frontend: il pannello lo serve questo stesso binario.
	baseURL string
}

func NewAdminService(admins *repository.AdminRepo, secret string, isDev bool, mailer Mailer, baseURL string) *AdminService {
	return &AdminService{
		admins:    admins,
		jwtSecret: []byte(secret),
		isDev:     isDev,
		mailer:    mailer,
		baseURL:   baseURL,
	}
}

func (s *AdminService) IsDev() bool { return s.isDev }

// SendLoginLink manda il link di accesso al pannello.
//
// Se l'indirizzo non è di nessun amministratore attivo non succede niente e non
// è un errore: la pagina di login risponde la stessa frase in ogni caso. Il
// contrario trasformerebbe il form in un modo per scoprire chi amministra
// l'installazione, provando indirizzi e guardando quale risposta cambia.
//
// Restituisce il token solo in sviluppo (come SendMagicLink per gli atleti),
// così si entra anche senza un provider di posta configurato.
func (s *AdminService) SendLoginLink(ctx context.Context, email string) (string, error) {
	email = NormalizeEmail(email)
	admin, err := s.admins.FindActiveByEmail(ctx, email)
	if err != nil {
		if s.isDev {
			fmt.Printf("[admin-login] %s non è un amministratore attivo — nessuna email inviata\n", email)
		}
		return "", nil
	}

	token, err := s.admins.StoreLoginToken(ctx, admin.ID)
	if err != nil {
		return "", err
	}
	link := s.loginURL(token)

	if s.mailer == nil {
		if !s.isDev {
			return "", fmt.Errorf("no email provider configured: set RESEND_API_KEY")
		}
		fmt.Printf("[admin-login] %s → %s\n", email, link)
		return token, nil
	}
	if err := s.mailer.SendAdminLoginLink(ctx, email, link); err != nil {
		return "", fmt.Errorf("send admin login link: %w", err)
	}
	if s.isDev {
		fmt.Printf("[admin-login] inviato a %s → %s\n", email, link)
		return token, nil
	}
	return "", nil
}

func (s *AdminService) loginURL(token string) string {
	return strings.TrimRight(s.baseURL, "/") + "/admin/accesso/verifica?token=" + url.QueryEscape(token)
}

// VerifyLoginToken consuma il token e apre la sessione. Restituisce
// l'amministratore e il JWT da mettere nel cookie.
func (s *AdminService) VerifyLoginToken(ctx context.Context, token string) (*model.Admin, string, error) {
	admin, err := s.admins.VerifyLoginToken(ctx, token)
	if err != nil {
		return nil, "", ErrNotAdmin
	}
	signed, err := s.IssueJWT(admin)
	if err != nil {
		return nil, "", err
	}
	// Un errore qui non deve impedire l'accesso: last_login_at è
	// un'informazione di comodo nell'elenco amministratori, non un requisito.
	if err := s.admins.TouchLogin(ctx, admin.ID); err != nil {
		fmt.Printf("[admin-login] last_login_at per %s: %v\n", admin.Email, err)
	}
	return admin, signed, nil
}

func (s *AdminService) IssueJWT(a *model.Admin) (string, error) {
	csrf, err := randomHex(16)
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"sub":  a.ID,
		"aud":  adminAudience,
		"csrf": csrf,
		"exp":  time.Now().Add(adminSessionTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.jwtSecret)
}

// ParseJWT convalida il cookie di sessione del pannello.
//
// jwt.WithAudience fa rifiutare dalla libreria tutto ciò che non porta
// `aud: elite-admin` — cioè ogni token emesso per l'app degli atleti, che è
// firmato con lo stesso segreto e altrimenti passerebbe la verifica.
func (s *AdminService) ParseJWT(tokenStr string) (AdminSession, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	}, jwt.WithAudience(adminAudience))
	if err != nil || !t.Valid {
		return AdminSession{}, fmt.Errorf("invalid admin token")
	}
	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return AdminSession{}, fmt.Errorf("invalid admin token")
	}
	sess := AdminSession{}
	sess.AdminID, _ = claims["sub"].(string)
	sess.CSRF, _ = claims["csrf"].(string)
	if sess.AdminID == "" || sess.CSRF == "" {
		return AdminSession{}, fmt.Errorf("invalid admin token")
	}
	return sess, nil
}

// ValidCSRF confronta il token del form con quello della sessione.
//
// A tempo costante: il confronto con == esce al primo byte diverso, e il tempo
// che ci mette dice quanti byte erano giusti. È il modo in cui un segreto si
// indovina un carattere alla volta.
func (sess AdminSession) ValidCSRF(formValue string) bool {
	return subtle.ConstantTimeCompare([]byte(sess.CSRF), []byte(formValue)) == 1
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

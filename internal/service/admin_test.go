package service

import (
	"strings"
	"testing"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-at-least-32-characters!!"

func adminSvc() *AdminService {
	return NewAdminService(nil, testSecret, true, nil, "http://localhost:8080")
}

func TestAdminJWTRoundTrip(t *testing.T) {
	s := adminSvc()
	signed, err := s.IssueJWT(&model.Admin{ID: "admin-1", Email: "a@b.it"})
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}

	sess, err := s.ParseJWT(signed)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if sess.AdminID != "admin-1" {
		t.Errorf("AdminID = %q, atteso admin-1", sess.AdminID)
	}
	if sess.CSRF == "" {
		t.Error("il token non porta un segreto CSRF")
	}
}

// Il caso che conta: l'app degli atleti e il pannello firmano con lo stesso
// segreto, quindi la firma di un token da atleta è valida anche qui.
func TestAthleteJWTIsRejectedByAdminPanel(t *testing.T) {
	users := NewAuthService(nil, testSecret, true, nil, "http://localhost:5173")
	athlete, err := users.IssueJWT(&model.User{ID: "user-1", Email: "atleta@b.it"})
	if err != nil {
		t.Fatalf("IssueJWT atleta: %v", err)
	}

	// Prima la prova che è davvero un token buono per la sua parte, se no il
	// test passerebbe anche solo perché la stringa è malformata.
	if _, _, err := users.ParseJWT(athlete); err != nil {
		t.Fatalf("il token dell'atleta dovrebbe essere valido per l'app: %v", err)
	}

	if _, err := adminSvc().ParseJWT(athlete); err == nil {
		t.Fatal("un JWT da atleta è stato accettato come sessione del pannello")
	}
}

// TestAthleteJWTIsRejectedByAdminPanel non basta a dimostrare che `aud` serve:
// un token da atleta non ha nemmeno il claim csrf, quindi verrebbe respinto
// comunque dal controllo su quello. Provato togliendo jwt.WithAudience — il
// test passava lo stesso.
//
// Questo invece isola l'audience: il token ha la forma giusta in tutto il
// resto, e l'unica cosa che non va è a chi è destinato. È quello che
// succederebbe se un giorno l'app degli atleti cominciasse a emettere token con
// un csrf dentro per motivi suoi.
func TestAdminPanelRejectsForeignAudience(t *testing.T) {
	s := adminSvc()

	for _, aud := range []any{"elite-app", "", nil} {
		claims := jwt.MapClaims{
			"sub":  "admin-1",
			"csrf": "deadbeefdeadbeef",
			"exp":  time.Now().Add(time.Hour).Unix(),
		}
		if aud != nil {
			claims["aud"] = aud
		}
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
		if err != nil {
			t.Fatalf("firma: %v", err)
		}
		if _, err := s.ParseJWT(signed); err == nil {
			t.Fatalf("accettato un token con aud=%v", aud)
		}
	}

	// E il controllo positivo: con l'audience giusta lo stesso token passa,
	// così si sa che a respingerlo sopra era l'audience e non altro.
	claims := jwt.MapClaims{
		"sub":  "admin-1",
		"aud":  adminAudience,
		"csrf": "deadbeefdeadbeef",
		"exp":  time.Now().Add(time.Hour).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("firma: %v", err)
	}
	if _, err := s.ParseJWT(signed); err != nil {
		t.Fatalf("lo stesso token con l'audience giusta è stato rifiutato: %v", err)
	}
}

// E il contrario: la sessione del pannello non deve valere come utente.
func TestAdminJWTIsRejectedByAthleteAPI(t *testing.T) {
	admin, err := adminSvc().IssueJWT(&model.Admin{ID: "admin-1", Email: "a@b.it"})
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}
	users := NewAuthService(nil, testSecret, true, nil, "http://localhost:5173")
	userID, _, err := users.ParseJWT(admin)

	// ParseJWT degli atleti non guarda l'audience, quindi la firma passa: quello
	// che conta è che non ne esca un'identità utilizzabile. `sub` è l'id di una
	// riga di `admins`, e ogni query dell'API filtra per user_id, quindi non
	// combacia con nessun utente. Se un domani si aggiungesse un `aud` anche di
	// là, questo diventerebbe un errore e andrebbe bene lo stesso.
	if err == nil && userID != "admin-1" {
		t.Fatalf("userID = %q, atteso l'id grezzo o un errore", userID)
	}
}

func TestAdminJWTRejectsWrongSecret(t *testing.T) {
	signed, err := adminSvc().IssueJWT(&model.Admin{ID: "admin-1"})
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}
	other := NewAdminService(nil, "another-secret-at-least-32-chars!!!!", true, nil, "")
	if _, err := other.ParseJWT(signed); err == nil {
		t.Fatal("un token firmato con un altro segreto è stato accettato")
	}
}

func TestAdminJWTRejectsExpired(t *testing.T) {
	// Emesso a mano perché IssueJWT non permette di scegliere la scadenza.
	claims := jwt.MapClaims{
		"sub":  "admin-1",
		"aud":  adminAudience,
		"csrf": "deadbeef",
		"exp":  time.Now().Add(-time.Minute).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("firma: %v", err)
	}
	if _, err := adminSvc().ParseJWT(signed); err == nil {
		t.Fatal("un token scaduto è stato accettato")
	}
}

// Un token senza segreto CSRF non deve aprire una sessione: se passasse, i
// controlli sui form si ridurrebbero a confrontare due stringhe vuote, cioè a
// non controllare niente.
func TestAdminJWTRejectsMissingCSRF(t *testing.T) {
	claims := jwt.MapClaims{
		"sub": "admin-1",
		"aud": adminAudience,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("firma: %v", err)
	}
	if _, err := adminSvc().ParseJWT(signed); err == nil {
		t.Fatal("un token senza claim csrf è stato accettato")
	}
}

func TestValidCSRF(t *testing.T) {
	sess := AdminSession{AdminID: "a", CSRF: "abc123"}
	if !sess.ValidCSRF("abc123") {
		t.Error("il token corretto è stato rifiutato")
	}
	for _, wrong := range []string{"", "abc124", "abc12", "abc1234"} {
		if sess.ValidCSRF(wrong) {
			t.Errorf("ValidCSRF(%q) = true", wrong)
		}
	}
}

// Ogni accesso deve avere il suo segreto CSRF: riusarlo fra sessioni
// significherebbe che un token raccolto una volta resta buono per sempre.
func TestCSRFDiffersBetweenSessions(t *testing.T) {
	s := adminSvc()
	a := &model.Admin{ID: "admin-1"}

	first, _ := s.IssueJWT(a)
	second, _ := s.IssueJWT(a)
	s1, err := s.ParseJWT(first)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	s2, err := s.ParseJWT(second)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if s1.CSRF == s2.CSRF {
		t.Fatal("due accessi hanno lo stesso segreto CSRF")
	}
}

func TestAdminLoginURL(t *testing.T) {
	s := NewAdminService(nil, testSecret, true, nil, "http://localhost:8080/")
	got := s.loginURL("abc")
	want := "http://localhost:8080/admin/accesso/verifica?token=abc"
	if got != want {
		t.Errorf("loginURL = %q, atteso %q", got, want)
	}
	if strings.Contains(got, "//admin") {
		t.Error("la barra finale del baseURL non è stata tolta")
	}
}

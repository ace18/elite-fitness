package repository

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/elitecoach/backend/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrAdminExists — l'indirizzo è già di un amministratore. Va distinto da un
// errore qualunque perché il pannello ci scrive sopra un messaggio preciso, e
// perché riattivare un amministratore disattivato è una cosa diversa dal
// crearne uno nuovo.
var ErrAdminExists = errors.New("admin already exists")

// ErrLastAdmin — si sta cercando di disattivare l'ultimo amministratore attivo.
// Lasciarlo fare significherebbe un'installazione senza nessuno che possa
// entrare nel pannello, recuperabile solo a mano dal database.
var ErrLastAdmin = errors.New("cannot disable the last active admin")

// adminTokenTTL — quanto vale un link di accesso al pannello. Più corto dei 15
// minuti degli atleti: qui dietro c'è la vista su tutti gli iscritti, e un link
// nella casella di posta è un link che qualcun altro può leggere.
const adminTokenTTL = 10 * time.Minute

type AdminRepo struct {
	db *pgxpool.Pool
}

func NewAdminRepo(db *pgxpool.Pool) *AdminRepo { return &AdminRepo{db: db} }

const adminColumns = `a.id, a.email, a.name, a.created_by, a.created_at,
                      a.last_login_at, a.disabled_at`

func scanAdmin(row pgx.Row, a *model.Admin) error {
	return row.Scan(&a.ID, &a.Email, &a.Name, &a.CreatedBy, &a.CreatedAt,
		&a.LastLoginAt, &a.DisabledAt)
}

// ---- amministratori --------------------------------------------------------

// Bootstrap crea il primo amministratore, e solo quello: se la tabella ha già
// una riga non fa niente e lo dice restituendo false.
//
// La condizione "tabella vuota" invece di "questo indirizzo non c'è" è
// deliberata. ADMIN_BOOTSTRAP_EMAIL resta impostata nell'ambiente per sempre
// (Coolify, .env), quindi verrebbe riletta a ogni riavvio: con un controllo per
// indirizzo, un amministratore disattivato o rimosso tornerebbe da solo al
// primo restart, e il pannello non avrebbe modo di toglierlo davvero.
func (r *AdminRepo) Bootstrap(ctx context.Context, email string) (bool, error) {
	tag, err := r.db.Exec(ctx,
		`INSERT INTO admins (email)
		 SELECT $1
		 WHERE NOT EXISTS (SELECT 1 FROM admins)`,
		normalizeEmail(email))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// Create inserisce un amministratore per conto di un altro. `createdBy` è
// l'unico modo di arrivare qui: non esiste una variante senza autore, perché
// non esiste un percorso in cui qualcuno si crei da sé.
func (r *AdminRepo) Create(ctx context.Context, email, createdBy string) (*model.Admin, error) {
	a := &model.Admin{}
	err := r.db.QueryRow(ctx,
		`INSERT INTO admins (email, created_by) VALUES ($1, $2)
		 RETURNING id, email, name, created_by, created_at, last_login_at, disabled_at`,
		normalizeEmail(email), createdBy,
	).Scan(&a.ID, &a.Email, &a.Name, &a.CreatedBy, &a.CreatedAt, &a.LastLoginAt, &a.DisabledAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrAdminExists
		}
		return nil, err
	}
	return a, nil
}

func (r *AdminRepo) FindByID(ctx context.Context, id string) (*model.Admin, error) {
	a := &model.Admin{}
	err := scanAdmin(r.db.QueryRow(ctx,
		`SELECT `+adminColumns+` FROM admins a WHERE a.id = $1`, id), a)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// FindActiveByEmail trova l'amministratore a cui emettere un link di accesso.
// I disattivati non risultano: è qui che la revoca diventa effettiva sul
// percorso di login, come FindByID lo è su quello di sessione.
func (r *AdminRepo) FindActiveByEmail(ctx context.Context, email string) (*model.Admin, error) {
	a := &model.Admin{}
	err := scanAdmin(r.db.QueryRow(ctx,
		`SELECT `+adminColumns+` FROM admins a
		 WHERE a.email = $1 AND a.disabled_at IS NULL`, normalizeEmail(email)), a)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// List restituisce tutti gli amministratori, attivi e non, con l'indirizzo di
// chi li ha creati. L'auto-join è LEFT: il primo amministratore non ha autore.
func (r *AdminRepo) List(ctx context.Context) ([]model.Admin, error) {
	rows, err := r.db.Query(ctx,
		`SELECT `+adminColumns+`, creator.email
		 FROM admins a
		 LEFT JOIN admins creator ON creator.id = a.created_by
		 ORDER BY a.disabled_at IS NOT NULL, a.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.Admin
	for rows.Next() {
		var a model.Admin
		if err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.CreatedBy, &a.CreatedAt,
			&a.LastLoginAt, &a.DisabledAt, &a.CreatedByEmail); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// SetDisabled attiva o disattiva un amministratore.
//
// La protezione sull'ultimo attivo sta nella query, non in un controllo prima:
// due richieste in parallelo che disattivano gli ultimi due amministratori
// passerebbero entrambe un controllo fatto a monte e lascerebbero fuori tutti.
// Qui la sottoquery viene valutata dentro la stessa istruzione, quindi la
// seconda non trova più il compagno da contare.
func (r *AdminRepo) SetDisabled(ctx context.Context, id string, disabled bool) error {
	if !disabled {
		_, err := r.db.Exec(ctx,
			`UPDATE admins SET disabled_at = NULL WHERE id = $1`, id)
		return err
	}
	tag, err := r.db.Exec(ctx,
		`UPDATE admins SET disabled_at = NOW()
		 WHERE id = $1 AND disabled_at IS NULL
		   AND EXISTS (
		     SELECT 1 FROM admins other
		     WHERE other.id <> admins.id AND other.disabled_at IS NULL
		   )`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// O era già disattivato, o è l'ultimo rimasto. Il secondo caso è
		// l'unico che valga la pena raccontare a chi ha premuto il pulsante.
		return ErrLastAdmin
	}
	return nil
}

func (r *AdminRepo) TouchLogin(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE admins SET last_login_at = NOW() WHERE id = $1`, id)
	return err
}

// ---- token di accesso ------------------------------------------------------

func (r *AdminRepo) StoreLoginToken(ctx context.Context, adminID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	_, err := r.db.Exec(ctx,
		`INSERT INTO admin_login_tokens (admin_id, token, expires_at) VALUES ($1, $2, $3)`,
		adminID, token, time.Now().Add(adminTokenTTL))
	return token, err
}

// VerifyLoginToken consuma il token e restituisce l'amministratore.
//
// Il consumo e la lettura stanno nella stessa istruzione: separarli lascerebbe
// una finestra in cui lo stesso link, aperto due volte in fretta (il
// prefetch di un client di posta basta), passa entrambe le volte.
//
// Il join pretende disabled_at IS NULL, quindi un link emesso e poi revocato
// non funziona più. Il token risulta comunque usato: è quello che vogliamo, un
// tentativo su un accesso revocato non deve lasciare in giro un token ancora
// valido.
func (r *AdminRepo) VerifyLoginToken(ctx context.Context, token string) (*model.Admin, error) {
	a := &model.Admin{}
	err := scanAdmin(r.db.QueryRow(ctx,
		`UPDATE admin_login_tokens t
		 SET used_at = NOW()
		 FROM admins a
		 WHERE t.token = $1
		   AND t.used_at IS NULL
		   AND t.expires_at > NOW()
		   AND a.id = t.admin_id
		   AND a.disabled_at IS NULL
		 RETURNING `+adminColumns, token), a)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (r *AdminRepo) DeleteExpiredTokensBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.db.Exec(ctx, `DELETE FROM admin_login_tokens WHERE expires_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---- atleti ----------------------------------------------------------------

// ListAthletes costruisce l'elenco della pagina principale in una query sola.
//
// I due LATERAL sono lì per questo: la variante ovvia — leggere gli utenti e
// poi, per ciascuno, programma e statistiche — costa una query per riga e
// diventa lenta esattamente quando l'installazione comincia ad andare bene.
//
// L'ordinamento mette in cima chi si è allenato di recente e in fondo chi non
// l'ha mai fatto (NULLS LAST). È l'ordine utile a chi guarda: gli iscritti che
// non hanno mai aperto l'app non sono la notizia del giorno.
func (r *AdminRepo) ListAthletes(ctx context.Context) ([]model.AthleteRow, error) {
	rows, err := r.db.Query(ctx,
		`SELECT u.id, u.email, u.name, u.created_at,
		        p.id, p.name, p.total_weeks, p.days_per_week, p.started_at,
		        s.last_at, s.total, s.week
		 FROM users u
		 LEFT JOIN LATERAL (
		   SELECT id, name, total_weeks, days_per_week, started_at
		   FROM user_programs
		   WHERE user_id = u.id AND is_active = TRUE
		   ORDER BY started_at DESC LIMIT 1
		 ) p ON TRUE
		 LEFT JOIN LATERAL (
		   SELECT MAX(completed_at) AS last_at,
		          COUNT(*)          AS total,
		          COUNT(*) FILTER (WHERE completed_at > NOW() - INTERVAL '7 days') AS week
		   FROM session_logs WHERE user_id = u.id
		 ) s ON TRUE
		 ORDER BY s.last_at DESC NULLS LAST, u.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.AthleteRow
	for rows.Next() {
		var a model.AthleteRow
		var progID, progName *string
		var totalWeeks, daysPerWeek *int
		var startedAt *time.Time
		// COUNT non è mai NULL, ma i LATERAL restano LEFT JOIN: per un utente
		// senza nessuna sessione la sottoquery produce comunque una riga con
		// COUNT = 0, quindi questi due scalari arrivano sempre valorizzati.
		var total, week int

		if err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.CreatedAt,
			&progID, &progName, &totalWeeks, &daysPerWeek, &startedAt,
			&a.LastSessionAt, &total, &week); err != nil {
			return nil, err
		}
		a.TotalSessions, a.Sessions7d = total, week

		if progID != nil && startedAt != nil {
			p := &model.AthleteProgram{
				ID: *progID, Name: derefString(progName),
				TotalWeeks: derefInt(totalWeeks), DaysPerWeek: derefInt(daysPerWeek),
				StartedAt: *startedAt,
			}
			// La settimana in corso è derivata da started_at, non letta: è la
			// stessa regola che applica l'app (vedi UserProgram.WeekAt), e
			// ricalcolarla qui evita che il pannello mostri un numero diverso
			// da quello che l'atleta ha sul telefono.
			up := &model.UserProgram{StartedAt: p.StartedAt, TotalWeeks: p.TotalWeeks}
			p.Week = up.WeekAt(time.Now())
			a.Program = p
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// FindAthlete legge l'intestazione della scheda atleta. Programma e statistiche
// li recuperano poi i repository che già esistono (ProgramRepo, SessionRepo):
// prendono l'id utente come parametro, quindi funzionano identici che a
// chiamarli sia l'atleta o il pannello.
func (r *AdminRepo) FindAthlete(ctx context.Context, userID string) (*model.AthleteRow, error) {
	a := &model.AthleteRow{}
	err := r.db.QueryRow(ctx,
		`SELECT u.id, u.email, u.name, u.created_at,
		        s.last_at, s.total, s.week
		 FROM users u
		 LEFT JOIN LATERAL (
		   SELECT MAX(completed_at) AS last_at,
		          COUNT(*)          AS total,
		          COUNT(*) FILTER (WHERE completed_at > NOW() - INTERVAL '7 days') AS week
		   FROM session_logs WHERE user_id = u.id
		 ) s ON TRUE
		 WHERE u.id = $1`, userID,
	).Scan(&a.ID, &a.Email, &a.Name, &a.CreatedAt,
		&a.LastSessionAt, &a.TotalSessions, &a.Sessions7d)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// ListAthleteSessions — lo storico allenamenti della scheda atleta.
func (r *AdminRepo) ListAthleteSessions(ctx context.Context, userID string, limit int) ([]model.SessionLog, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, name, duration_min, total_volume, total_sets, avg_rpe,
		        session_rpe, completed_at
		 FROM session_logs
		 WHERE user_id = $1
		 ORDER BY completed_at DESC
		 LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.SessionLog
	for rows.Next() {
		var s model.SessionLog
		if err := rows.Scan(&s.ID, &s.Name, &s.DurationMin, &s.TotalVolume,
			&s.TotalSets, &s.AvgRPE, &s.SessionRPE, &s.CompletedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetAthleteSession legge un allenamento con le sue serie.
//
// user_id è nella WHERE e non solo l'id della sessione: senza, un id indovinato
// o copiato da un'altra scheda mostrerebbe l'allenamento di un altro atleta
// sotto l'intestazione sbagliata. Nel pannello chi guarda ha comunque accesso a
// tutti, ma una pagina che dice "Marco" mostrando i dati di Luca è un bug che
// non si nota finché non conta.
func (r *AdminRepo) GetAthleteSession(ctx context.Context, userID, sessionID string) (*model.SessionLog, error) {
	s := &model.SessionLog{}
	err := r.db.QueryRow(ctx,
		`SELECT id, name, duration_min, total_volume, total_sets, avg_rpe,
		        session_rpe, completed_at
		 FROM session_logs
		 WHERE id = $1 AND user_id = $2`, sessionID, userID,
	).Scan(&s.ID, &s.Name, &s.DurationMin, &s.TotalVolume, &s.TotalSets,
		&s.AvgRPE, &s.SessionRPE, &s.CompletedAt)
	if err != nil {
		return nil, err
	}

	// L'ordine è per esercizio e poi per numero di serie perché non c'è modo di
	// ricostruire quello vero: set_logs non ha né una colonna d'ordine né un
	// timestamp, e la chiave primaria è un UUID casuale, quindi ORDER BY id
	// darebbe le serie mescolate. Raggruppare per esercizio è comunque il modo
	// in cui si legge un allenamento; si perde solo la sequenza con cui gli
	// esercizi si sono susseguiti nella seduta.
	rows, err := r.db.Query(ctx,
		`SELECT exercise_name, set_number,
		        COALESCE(weight, 0), COALESCE(reps, 0), rpe, is_pr
		 FROM set_logs WHERE session_id = $1
		 ORDER BY exercise_name, set_number`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sl model.SetLog
		if err := rows.Scan(&sl.ExerciseName, &sl.SetNumber, &sl.Weight,
			&sl.Reps, &sl.RPE, &sl.IsPR); err != nil {
			return nil, err
		}
		s.Sets = append(s.Sets, sl)
	}
	return s, rows.Err()
}

// isUniqueViolation riconosce il 23505 di Postgres. Serve a distinguere "questo
// indirizzo è già amministratore" — un esito normale, con un suo messaggio —
// da un errore vero del database.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

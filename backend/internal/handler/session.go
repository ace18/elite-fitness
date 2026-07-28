package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/elitecoach/backend/internal/middleware"
	"github.com/elitecoach/backend/internal/model"
	"github.com/elitecoach/backend/internal/repository"
	"github.com/elitecoach/backend/internal/service"
)

type SessionHandler struct {
	sessions  *repository.SessionRepo
	completer *service.ProgramCompleter
}

func NewSessionHandler(sessions *repository.SessionRepo, completer *service.ProgramCompleter) *SessionHandler {
	return &SessionHandler{sessions: sessions, completer: completer}
}

func (h *SessionHandler) SaveSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	var s model.SessionLog
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		jsonError(w, "invalid body", http.StatusBadRequest)
		return
	}
	s.UserID = userID

	// Il programma lo decide il server, non il client: il programId del body
	// non è verificato e potrebbe puntare al programma di un altro utente. È
	// anche ciò su cui si conta per capire se il programma è finito, quindi
	// deve essere attendibile.
	program := h.completer.ActiveProgram(r.Context(), userID)
	if program != nil {
		s.ProgramID = &program.ID
	} else {
		s.ProgramID = nil
	}

	if err := h.sessions.SaveSession(r.Context(), &s); err != nil {
		jsonError(w, "failed to save session", http.StatusInternalServerError)
		return
	}

	// Era l'ultima sessione dell'ultima settimana? Se sì il programma si chiude
	// qui. Un errore non deve far fallire il salvataggio: la sessione è già
	// registrata e per l'utente è quello che conta.
	completed, err := h.completer.CompleteIfFinished(r.Context(), program)
	if err != nil && program != nil {
		log.Printf("complete program %s: %v", program.ID, err)
	}

	jsonOK(w, map[string]any{"ok": true, "id": s.ID, "programCompleted": completed})
}

func (h *SessionHandler) GetLastSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r)
	s, err := h.sessions.GetLastSession(r.Context(), userID)
	if err != nil {
		jsonError(w, "no sessions yet", http.StatusNotFound)
		return
	}
	jsonOK(w, s)
}

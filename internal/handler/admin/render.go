package admin

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//go:embed templates/*.html static/*
var assets embed.FS

// pages — un template per pagina, ognuno montato dentro il layout.
//
// Vanno elencati qui perché ogni pagina definisce il proprio blocco "content"
// e i blocchi non possono convivere nello stesso set: parsarli tutti insieme
// lascerebbe valido solo l'ultimo. Ogni voce diventa un set a sé, layout più
// la sua pagina.
var pages = []string{"login", "roster", "athlete", "session", "admins",
	"exercises", "programs", "program", "day", "error"}

// devTemplateDir — da dove rileggere i template in sviluppo, relativo alla
// cartella da cui si lancia il server (la radice del repo, con
// `go run ./cmd/server`).
//
// Serve una sorgente su disco perché il ricaricamento a caldo altrimenti non
// ricarica niente: `assets` è un embed.FS, cioè una copia dei file fatta al
// momento della compilazione. Rileggerlo restituisce sempre gli stessi byte, e
// il ricaricamento sembrerebbe funzionare — la pagina si ridisegna — mentre in
// realtà mostra i template di prima. Un modo perfetto per passare un'ora a
// modificare un file convinti che il problema sia altrove.
const devTemplateDir = "internal/handler/admin"

// renderer tiene i set di template. In sviluppo li ricostruisce a ogni
// richiesta leggendo da disco: senza, ogni ritocco a un .html richiederebbe di
// ricompilare e riavviare.
type renderer struct {
	sets map[string]*template.Template
	// devFS è nil fuori dallo sviluppo, e anche in sviluppo se il server è
	// stato lanciato da un'altra cartella (un binario compilato, per esempio).
	// In quel caso si serve quello che c'è compilato dentro, che è comunque
	// giusto — si perde solo il ricaricamento.
	devFS fs.FS
}

func newRenderer(isDev bool) (*renderer, error) {
	r := &renderer{}
	if isDev {
		if _, err := os.Stat(filepath.Join(devTemplateDir, "templates", "layout.html")); err == nil {
			r.devFS = os.DirFS(devTemplateDir)
			log.Printf("admin: template ricaricati da %s a ogni richiesta", devTemplateDir)
		}
	}
	sets, err := parseTemplates(assets)
	if err != nil {
		return nil, err
	}
	r.sets = sets
	return r, nil
}

// readAsset legge un file statico, da disco in sviluppo e dall'embed altrove.
func (rn *renderer) readAsset(name string) ([]byte, error) {
	if rn.devFS != nil {
		if b, err := fs.ReadFile(rn.devFS, name); err == nil {
			return b, nil
		}
		// Se su disco non c'è, si ricade sull'embed invece di dare 404: è
		// sempre meglio di una pagina senza foglio di stile.
	}
	return assets.ReadFile(name)
}

func parseTemplates(src fs.FS) (map[string]*template.Template, error) {
	sets := make(map[string]*template.Template, len(pages))
	for _, p := range pages {
		// layout per primo: la pagina, parsata dopo, sovrascrive i blocchi che
		// ridefinisce (il titolo) e riempie quelli che il layout lascia vuoti.
		t, err := template.New("layout.html").Funcs(funcs).ParseFS(src,
			"templates/layout.html", "templates/"+p+".html")
		if err != nil {
			return nil, fmt.Errorf("template %s: %w", p, err)
		}
		sets[p] = t
	}
	return sets, nil
}

// render scrive una pagina. Il template viene eseguito prima in memoria e poi
// copiato nella risposta: eseguendo direttamente sul ResponseWriter, un errore
// a metà template (un campo nil dereferenziato in un ramo raro) uscirebbe come
// 200 con mezza pagina dentro, e il browser mostrerebbe un layout troncato
// senza che nulla segnali il problema.
func (rn *renderer) render(w http.ResponseWriter, status int, page string, data any) {
	sets := rn.sets
	if rn.devFS != nil {
		fresh, err := parseTemplates(rn.devFS)
		if err != nil {
			log.Printf("admin: ricarica template: %v", err)
			http.Error(w, "errore nei template: "+err.Error(), http.StatusInternalServerError)
			return
		}
		sets = fresh
	}

	t, ok := sets[page]
	if !ok {
		log.Printf("admin: template %q inesistente", page)
		http.Error(w, "pagina non disponibile", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout.html", data); err != nil {
		log.Printf("admin: render %s: %v", page, err)
		http.Error(w, "errore nella pagina", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Le pagine mostrano dati di persone: non devono restare nella cache del
	// browser dopo il logout, né tantomeno su un proxy per strada.
	w.Header().Set("Cache-Control", "no-store, private")
	w.WriteHeader(status)
	buf.WriteTo(w)
}

// ---- funzioni dei template -------------------------------------------------

var months = [...]string{"", "gen", "feb", "mar", "apr", "mag", "giu",
	"lug", "ago", "set", "ott", "nov", "dic"}

// I formattatori numerici prendono `any` e non float64 perché nel database
// mezze colonne sono annullabili e arrivano qui come *float64 (durata, volume,
// RPE medio). Con una firma tipata ogni uso richiederebbe di dereferenziare nel
// template — cosa che i template non sanno fare — oppure un campo appiattito in
// più per ciascuna. Meglio un solo punto che sa gestire il nil.
var funcs = template.FuncMap{
	"data":      formatDate,
	"dataOra":   formatDateTime,
	"quando":    relativeDay,
	"kg":        func(v any) string { return numOr(v, 1, "—", " kg") },
	"num":       func(v any) string { return numOr(v, 1, "—", "") },
	"volume":    formatVolume,
	"delta":     formatDelta,
	"deltaPct":  formatDeltaPct,
	"livello":   level,
	"sparkline": sparkline,
	"haSerie":   hasSeries,
	"iniziali":  initials,
	"giorno":    dayName,
	"giorni":    weekDays,
	"pct":       loadPercent,
	"eq":        func(a, b any) bool { return fmt.Sprint(a) == fmt.Sprint(b) },
}

// weekdayNames — day_of_week è 0..6 con 0 = lunedì, come nelle template seminate
// dalla 004 (lo str-5x5 sta a 0, 2, 4: lunedì, mercoledì, venerdì).
var weekdayNames = [...]string{"lunedì", "martedì", "mercoledì", "giovedì",
	"venerdì", "sabato", "domenica"}

func dayName(n int) string {
	if n < 0 || n >= len(weekdayNames) {
		return "?"
	}
	return weekdayNames[n]
}

// weekDays serve ai menù a tendina: restituisce gli indici, il nome lo mette il
// template con `giorno`.
func weekDays() []int { return []int{0, 1, 2, 3, 4, 5, 6} }

// loadPercent riporta la frazione salvata (0.65) alla percentuale che si scrive
// nei form (65). Il database tiene la frazione perché è quella che moltiplica il
// massimale; le persone scrivono percentuali.
func loadPercent(v *float64) string {
	if v == nil {
		return ""
	}
	return decimal(*v*100, 1)
}

// hasSeries dice se c'è abbastanza roba da disegnare. Serve perché
// GetProgressMetrics riempie la serie solo per il peso corporeo: quelle del
// massimale stimato e del volume sono sempre vuote, e senza questo controllo la
// pagina mostrerebbe due riquadri di grafico vuoti che sembrano un errore.
func hasSeries(series []float64) bool { return len(series) >= 2 }

// toFloat srotola quello che i template possono passare a un formattatore:
// float64, *float64 (colonne annullabili), int e *int (durate, ripetizioni).
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case nil:
		return 0, false
	case float64:
		return n, true
	case *float64:
		if n == nil {
			return 0, false
		}
		return *n, true
	case int:
		return float64(n), true
	case *int:
		if n == nil {
			return 0, false
		}
		return float64(*n), true
	default:
		return 0, false
	}
}

func numOr(v any, places int, empty, suffix string) string {
	f, ok := toFloat(v)
	if !ok {
		return empty
	}
	return decimal(f, places) + suffix
}

// level traduce i livelli, che nel database stanno in inglese perché ci sono
// arrivati dalle template dei piani (migrazione 002). Tradurli qui e non nel
// database: sono l'etichetta di una riga condivisa con l'app, che ha già i suoi
// cataloghi di traduzioni e li userebbe comunque a modo suo.
func level(s string) string {
	switch strings.ToLower(s) {
	case "beginner":
		return "Principiante"
	case "intermediate":
		return "Intermedio"
	case "advanced":
		return "Avanzato"
	default:
		return s
	}
}

// decimal formatta con la virgola, che è come si scrivono i numeri in italiano.
// strconv usa il punto e non ha modo di cambiarlo.
func decimal(v float64, places int) string {
	s := strconv.FormatFloat(v, 'f', places, 64)
	return strings.Replace(s, ".", ",", 1)
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	t = t.Local()
	return fmt.Sprintf("%d %s %d", t.Day(), months[int(t.Month())], t.Year())
}

func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	t = t.Local()
	return fmt.Sprintf("%d %s %d, %02d:%02d",
		t.Day(), months[int(t.Month())], t.Year(), t.Hour(), t.Minute())
}

// relativeDay — "oggi", "ieri", "3 giorni fa". Conta i giorni di calendario e
// non le ore: un allenamento delle 23 di ieri è "ieri" anche se sono passate
// due ore, ed è così che lo direbbe chiunque.
//
// Accetta sia time.Time sia *time.Time: l'ultimo allenamento di un atleta può
// non esserci (puntatore nil), quello di una seduta registrata c'è sempre.
func relativeDay(v any) string {
	var then time.Time
	switch t := v.(type) {
	case time.Time:
		then = t
	case *time.Time:
		if t == nil {
			return "mai"
		}
		then = *t
	default:
		return "mai"
	}
	if then.IsZero() {
		return "mai"
	}

	now := time.Now().Local()
	then = then.Local()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	day := time.Date(then.Year(), then.Month(), then.Day(), 0, 0, 0, 0, then.Location())
	days := int(today.Sub(day).Hours() / 24)

	switch {
	case days <= 0:
		return "oggi"
	case days == 1:
		return "ieri"
	case days < 30:
		return fmt.Sprintf("%d giorni fa", days)
	case days < 60:
		return "un mese fa"
	case days < 365:
		return fmt.Sprintf("%d mesi fa", days/30)
	default:
		return formatDate(then)
	}
}

// formatDeltaPct — le variazioni che il database calcola in percentuale.
//
// Non tutti i Delta di ProgressMetrics sono la stessa cosa: quello del peso
// corporeo è una differenza in chili (weights[0] - weights[n]), quello del
// volume è una percentuale settimana su settimana. Renderli con la stessa
// funzione faceva leggere "+1,0" a un allenatore come un chilo in più, mentre
// era l'uno per cento.
func formatDeltaPct(v float64) string {
	if v == 0 {
		return "—"
	}
	if v > 0 {
		return "+" + decimal(v, 1) + "%"
	}
	return decimal(v, 1) + "%"
}

// formatVolume — il volume di un allenamento è dell'ordine delle tonnellate, e
// "12.480,0 kg" si legge peggio di "12,5 t".
//
// Attenzione: vale per i totali di una seduta (session_logs.total_volume), che
// sono in chili. ProgressMetrics.Volume.Value NO — arriva già diviso per mille
// (unit "k kg", vedi GetProgressMetrics), quindi lì va usato `num` con la t
// scritta a mano.
func formatVolume(v any) string {
	kg, ok := toFloat(v)
	if !ok {
		return "—"
	}
	if kg >= 1000 {
		return decimal(kg/1000, 1) + " t"
	}
	return decimal(kg, 0) + " kg"
}

// formatDelta antepone il segno: senza, "+0,4" e "-0,4" si distinguono solo
// guardando bene, e in una tabella di numeri è la differenza che conta.
func formatDelta(v float64) string {
	if v == 0 {
		return "—"
	}
	if v > 0 {
		return "+" + decimal(v, 1)
	}
	return decimal(v, 1)
}

// sparkline disegna un SVG dalla serie, senza JavaScript.
//
// È il pezzo che si teme di perdere passando dal frontend a un pannello reso
// dal server, e in realtà è più semplice così: nessuna libreria, nessuna
// idratazione, e la pagina si stampa.
func sparkline(series []float64) template.HTML {
	const w, h, pad = 220.0, 44.0, 3.0
	if len(series) < 2 {
		return template.HTML(`<svg class="spark" viewBox="0 0 220 44" role="presentation"></svg>`)
	}

	min, max := series[0], series[0]
	for _, v := range series {
		min, max = math.Min(min, v), math.Max(max, v)
	}
	// Serie piatta: senza questo il denominatore è zero e i punti finiscono a
	// NaN, che l'SVG rende come una linea che sparisce.
	span := max - min
	if span == 0 {
		span = 1
		min -= 0.5
	}

	var b strings.Builder
	stepX := (w - 2*pad) / float64(len(series)-1)
	for i, v := range series {
		if i > 0 {
			b.WriteByte(' ')
		}
		x := pad + float64(i)*stepX
		// y invertita: nell'SVG cresce verso il basso, nei grafici verso l'alto.
		y := h - pad - ((v-min)/span)*(h-2*pad)
		fmt.Fprintf(&b, "%.1f,%.1f", x, y)
	}

	return template.HTML(fmt.Sprintf(
		`<svg class="spark" viewBox="0 0 220 44" preserveAspectRatio="none" role="presentation">`+
			`<polyline points="%s" fill="none" stroke="currentColor" stroke-width="2" `+
			`stroke-linecap="round" stroke-linejoin="round"/></svg>`, b.String()))
}

// initials — le iniziali per il pallino dell'elenco. Da un'email prende la
// prima lettera, che è meglio di niente quando il nome non c'è.
func initials(name, email string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		if email == "" {
			return "?"
		}
		return strings.ToUpper(email[:1])
	}
	parts := strings.Fields(name)
	if len(parts) == 1 {
		return strings.ToUpper(parts[0][:1])
	}
	return strings.ToUpper(parts[0][:1] + parts[len(parts)-1][:1])
}

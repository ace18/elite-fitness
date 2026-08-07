package service

import (
	"testing"

	"github.com/elitecoach/backend/internal/repository"
)

func rpe(v float64) *float64 { return &v }

// Dentro un blocco lo schema non cambia, quindi c'è sempre una serie
// confrontabile e si autoregola sull'RPE come si è sempre fatto.
func TestSuggestNextAutoregulatesWithinABlock(t *testing.T) {
	for _, tc := range []struct {
		name string
		last float64
		r    *float64
		want float64
	}{
		{"facile: si sale", 100, rpe(6.5), 102.5},
		{"al limite: si sale", 100, rpe(7), 102.5},
		{"giusto: si resta", 100, rpe(8), 100},
		{"quasi cedimento: si scende", 100, rpe(9.5), 97.5},
		{"cedimento: si scende", 100, rpe(10), 97.5},
		{"senza RPE non si muove", 100, nil, 100},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := repository.ExerciseHistory{Last: tc.last, LastRPE: tc.r, Recent1RM: 130}
			if got := suggestNext(h, 5, "compound"); got != tc.want {
				t.Errorf("suggestNext = %v, volevo %v", got, tc.want)
			}
		})
	}
}

// Il passo è una percentuale del carico, non 2.5 kg per tutti. Su un fondamentale
// pesante deve valere più di uno scatto, su un complementare leggero deve
// scendere al taglio più piccolo invece di sparare il 12% in una volta.
func TestProgressionStepScalesWithTheLoad(t *testing.T) {
	easy := rpe(6.5)
	for _, tc := range []struct {
		name     string
		weight   float64
		category string
		want     float64 // peso proposto dopo una serie facile
	}{
		{"stacco pesante: uno scatto non basta", 200, "compound", 205},
		{"squat medio: uno scatto", 100, "compound", 102.5},
		{"panca leggera: minimo caricabile", 60, "compound", 62.5},
		{"alzata laterale: taglio piccolo", 20, "isolation", 21.25},
		{"curl medio: percentuale piena", 80, "isolation", 82.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := repository.ExerciseHistory{Last: tc.weight, LastRPE: easy}
			if got := suggestNext(h, 8, tc.category); got != tc.want {
				t.Errorf("da %v kg (%s) -> %v, volevo %v", tc.weight, tc.category, got, tc.want)
			}
		})
	}
}

// Lo stesso peso su un fondamentale e su un complementare non si muove allo
// stesso modo: è tutto il punto della categoria.
func TestProgressionStepDependsOnCategory(t *testing.T) {
	h := repository.ExerciseHistory{Last: 20, LastRPE: rpe(6.5)}
	compound := suggestNext(h, 8, "compound")
	isolation := suggestNext(h, 8, "isolation")
	if compound <= isolation {
		t.Fatalf("compound %v, isolation %v: il complementare deve salire di meno", compound, isolation)
	}
	if compound != 22.5 || isolation != 21.25 {
		t.Errorf("compound %v (volevo 22.5), isolation %v (volevo 21.25)", compound, isolation)
	}
}

// Un carico assurdo — uno zero di troppo battuto a mano — non deve tradursi in
// un incremento a due cifre proposto con la faccia seria.
func TestProgressionStepIsCapped(t *testing.T) {
	h := repository.ExerciseHistory{Last: 1000, LastRPE: rpe(6.5)}
	if got := suggestNext(h, 5, "compound"); got != 1010 {
		t.Errorf("da 1000 kg -> %v, volevo 1010 (passo limitato a %v)", got, maxStep)
	}
}

// Il passo scala anche quando si scende, se no si sale in percentuale e si
// scende di un'inezia.
func TestProgressionStepScalesDownwardToo(t *testing.T) {
	h := repository.ExerciseHistory{Last: 200, LastRPE: rpe(10)}
	if got := suggestNext(h, 5, "compound"); got != 195 {
		t.Errorf("da 200 kg a RPE 10 -> %v, volevo 195", got)
	}
}

// Un peso più leggero del passo non deve finire a zero o sottozero.
func TestProgressionStepNeverGoesNegative(t *testing.T) {
	h := repository.ExerciseHistory{Last: 1, LastRPE: rpe(10)}
	if got := suggestNext(h, 12, "isolation"); got <= 0 {
		t.Errorf("da 1 kg a RPE 10 -> %v, non deve scendere a zero o sotto", got)
	}
}

// Il caso per cui esiste tutto questo: primo giorno di un blocco nuovo. A
// queste ripetizioni non si è mai lavorato (Last = 0), quindi il peso si stima
// dal massimale recente invece di riproporre quello dello schema precedente.
//
// Concretamente: chi chiudeva il blocco 5×5 a 100 kg ha un 1RM stimato di circa
// 116 kg, che per un triplo vale ~105 — non 100.
func TestSuggestNextEstimatesAtTheStartOfANewBlock(t *testing.T) {
	h := repository.ExerciseHistory{
		Last:      0, // niente di confrontabile a 3 ripetizioni
		Recent1RM: 100 * (1 + 5.0/30),
	}

	got := suggestNext(h, 3, "compound")
	if got != 105 {
		t.Errorf("primo giorno a 3 ripetizioni = %v, volevo 105", got)
	}
	// Il punto: più del peso a cui si tiravano i cinque.
	if got <= 100 {
		t.Errorf("la stima (%v) non supera il carico del blocco precedente (100)", got)
	}
}

// E nell'altro verso: passando a uno schema più leggero il peso deve scendere,
// non restare quello dei tripli.
func TestSuggestNextComesDownForHigherReps(t *testing.T) {
	h := repository.ExerciseHistory{Recent1RM: 120} // ~110 kg per un triplo

	got := suggestNext(h, 12, "compound")
	if got >= 100 {
		t.Errorf("a 12 ripetizioni = %v, mi aspettavo un calo netto sotto i 100", got)
	}
	if got != 85 {
		t.Errorf("a 12 ripetizioni = %v, volevo 85", got)
	}
}

// Senza storico non si propone niente: meglio un campo vuoto che un numero
// inventato su cui qualcuno si carica.
func TestSuggestNextProposesNothingWithoutHistory(t *testing.T) {
	if got := suggestNext(repository.ExerciseHistory{}, 5, "compound"); got != 0 {
		t.Errorf("senza storico = %v, volevo 0", got)
	}
}

// Quel che esce va caricato su un bilanciere: sempre un multiplo di 2.5, e
// arrotondato per difetto perché è una stima.
func TestSuggestNextRoundsToLoadableWeight(t *testing.T) {
	for _, oneRM := range []float64{100, 117.3, 143.9, 88.2, 201.7} {
		got := suggestNext(repository.ExerciseHistory{Recent1RM: oneRM}, 5, "compound")
		if inc := loadIncrement("compound"); got/inc != float64(int(got/inc)) {
			t.Errorf("1RM %v -> %v, non è un multiplo di %v", oneRM, got, inc)
		}
		if exact := weightForReps(oneRM, 5); got > exact {
			t.Errorf("1RM %v -> %v, arrotondato in su rispetto a %v", oneRM, got, exact)
		}
	}
}

// Le fasce dicono quali serie sono confrontabili fra loro. Un triplo e un
// cinque no; due serie da otto e da dieci sì.
func TestRepBandBounds(t *testing.T) {
	sameBand := func(a, b int) bool {
		lo, hi := repository.RepBandBounds(a)
		return b >= lo && b <= hi
	}
	for _, tc := range []struct {
		a, b int
		want bool
	}{
		{5, 5, true},
		{8, 10, true},   // stessa fascia ipertrofia
		{12, 15, true},  // stessa fascia resistenza
		{20, 25, true},  // oltre la scala, tutte insieme
		{3, 5, false},   // forza massimale contro forza
		{5, 8, false},   // il salto che rompeva suggestNext
		{10, 12, false}, // fascia diversa
		{6, 7, false},   // adiacenti ma di là dal confine
	} {
		if got := sameBand(tc.a, tc.b); got != tc.want {
			t.Errorf("%d e %d nella stessa fascia = %v, volevo %v", tc.a, tc.b, got, tc.want)
		}
	}

	// La relazione deve valere nei due sensi, se no "confrontabile" dipende da
	// quale delle due serie si guarda per prima.
	for a := 1; a <= 30; a++ {
		for b := 1; b <= 30; b++ {
			if sameBand(a, b) != sameBand(b, a) {
				t.Fatalf("fasce asimmetriche fra %d e %d", a, b)
			}
		}
	}
}

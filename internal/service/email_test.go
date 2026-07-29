package service

import "strings"
import "testing"

func TestMagicLinkCopyDefaultsToItalian(t *testing.T) {
	// L'italiano è la lingua predefinita dell'app: chi non manda un locale
	// (client vecchio, callback OAuth) deve ricevere l'email in italiano.
	for _, locale := range []string{"", "de", "xx", "IT"} {
		if got := magicLinkCopy(locale).subject; got != emailCopies["it"].subject {
			t.Errorf("locale %q → subject %q, want the Italian one", locale, got)
		}
	}
}

func TestMagicLinkCopySelectsLocale(t *testing.T) {
	if magicLinkCopy("en").subject != emailCopies["en"].subject {
		t.Error(`locale "en" did not select the English copy`)
	}
	if magicLinkCopy("it").subject != emailCopies["it"].subject {
		t.Error(`locale "it" did not select the Italian copy`)
	}
}

func TestMagicLinkBodiesCarryTheLinkAndLocale(t *testing.T) {
	const link = "https://elite.app/login?token=abc123"
	for _, locale := range []string{"it", "en"} {
		c := magicLinkCopy(locale)
		for name, body := range map[string]string{
			"html": magicLinkHTML(link, c),
			"text": magicLinkText(link, c),
		} {
			if !strings.Contains(body, link) {
				t.Errorf("%s/%s is missing the sign-in link", locale, name)
			}
			if !strings.Contains(body, c.heading) {
				t.Errorf("%s/%s is missing the localised heading", locale, name)
			}
		}
	}
}

// Le due lingue devono avere tutti i campi pieni: un campo vuoto passerebbe
// inosservato e manderebbe un'email con un pulsante senza testo.
func TestEmailCopiesAreComplete(t *testing.T) {
	for locale, c := range emailCopies {
		for name, v := range map[string]string{
			"subject": c.subject, "heading": c.heading, "intro": c.intro,
			"button": c.button, "orPaste": c.orPaste, "ignore": c.ignore,
		} {
			if strings.TrimSpace(v) == "" {
				t.Errorf("%s: %s is empty", locale, name)
			}
		}
	}
}

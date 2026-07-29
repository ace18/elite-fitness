package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"time"
)

// Mailer spedisce le email transazionali. L'interfaccia esiste perché
// AuthService possa girare anche senza provider (in dev) e per i test.
type Mailer interface {
	SendMagicLink(ctx context.Context, to, link, locale string) error
}

const resendEndpoint = "https://api.resend.com/emails"

// ResendMailer parla con l'API di Resend (POST /emails). È un'unica chiamata
// HTTP autenticata con Bearer token: non vale la pena tirare dentro l'SDK.
type ResendMailer struct {
	apiKey string
	from   string
	client *http.Client
}

// NewResendMailer costruisce il mailer. `from` deve essere un indirizzo su un
// dominio verificato in Resend, nella forma "ELITE <no-reply@dominio.com>".
func NewResendMailer(apiKey, from string) *ResendMailer {
	return &ResendMailer{
		apiKey: apiKey,
		from:   from,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type resendEmail struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
	Text    string   `json:"text"`
}

func (m *ResendMailer) SendMagicLink(ctx context.Context, to, link, locale string) error {
	c := magicLinkCopy(locale)
	return m.send(ctx, resendEmail{
		From:    m.from,
		To:      []string{to},
		Subject: c.subject,
		HTML:    magicLinkHTML(link, c),
		Text:    magicLinkText(link, c),
	})
}

// emailCopy — i testi dell'email. Vivono qui e non nei cataloghi del frontend
// perché l'email parte dal server, prima ancora che l'utente abbia un account.
type emailCopy struct {
	subject, heading, intro, button, orPaste, ignore string
}

var emailCopies = map[string]emailCopy{
	"it": {
		subject: "Il tuo link di accesso a ELITE",
		heading: "Accedi a ELITE",
		intro:   "Tocca il pulsante qui sotto per entrare. Il link è valido una sola volta e scade fra 15 minuti.",
		button:  "Accedi →",
		orPaste: "Oppure incolla questo link nel browser:",
		ignore:  "Se non hai richiesto tu l'accesso, puoi ignorare questa email.",
	},
	"en": {
		subject: "Your ELITE sign-in link",
		heading: "Sign in to ELITE",
		intro:   "Tap the button below to sign in. The link works once and expires in 15 minutes.",
		button:  "Sign in →",
		orPaste: "Or paste this link into your browser:",
		ignore:  "If you didn't request this, you can ignore this email.",
	},
}

// magicLinkCopy sceglie la lingua, con l'italiano come predefinito — è la
// lingua di default dell'app, quindi anche quella di chi non la specifica.
func magicLinkCopy(locale string) emailCopy {
	if c, ok := emailCopies[locale]; ok {
		return c
	}
	return emailCopies["it"]
}

func (m *ResendMailer) send(ctx context.Context, email resendEmail) error {
	payload, err := json.Marshal(email)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		// Il corpo d'errore di Resend è JSON con name/message: utile nei log,
		// non va mai rimandato al client (può contenere dettagli d'account).
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("resend: %s: %s", res.Status, bytes.TrimSpace(body))
	}
	return nil
}

// Il token è esadecimale e l'URL lo costruiamo noi, ma l'escape resta: se un
// giorno il link diventa parametrizzabile non vogliamo un'injection nell'HTML.
func magicLinkHTML(link string, c emailCopy) string {
	safe := html.EscapeString(link)
	return `<!doctype html>
<html>
  <body style="margin:0;padding:0;background:#f5f5f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f5f5f7;padding:40px 16px;">
      <tr>
        <td align="center">
          <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:480px;background:#ffffff;border-radius:16px;padding:36px 32px;">
            <tr>
              <td>
                <h1 style="margin:0 0 12px;font-size:24px;line-height:1.25;color:#111;">` + html.EscapeString(c.heading) + `</h1>
                <p style="margin:0 0 28px;font-size:15px;line-height:1.5;color:#555;">
                  ` + html.EscapeString(c.intro) + `
                </p>
                <a href="` + safe + `" style="display:inline-block;background:#111;color:#fff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 28px;border-radius:12px;">
                  ` + html.EscapeString(c.button) + `
                </a>
                <p style="margin:28px 0 0;font-size:13px;line-height:1.5;color:#888;">
                  ` + html.EscapeString(c.orPaste) + `<br />
                  <span style="color:#555;word-break:break-all;">` + safe + `</span>
                </p>
                <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#888;">
                  ` + html.EscapeString(c.ignore) + `
                </p>
              </td>
            </tr>
          </table>
        </td>
      </tr>
    </table>
  </body>
</html>`
}

func magicLinkText(link string, c emailCopy) string {
	return c.heading + "\n\n" + c.intro + "\n\n" + link + "\n\n" + c.ignore + "\n"
}

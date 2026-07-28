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
	SendMagicLink(ctx context.Context, to, link string) error
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

func (m *ResendMailer) SendMagicLink(ctx context.Context, to, link string) error {
	return m.send(ctx, resendEmail{
		From:    m.from,
		To:      []string{to},
		Subject: "Your ELITE sign-in link",
		HTML:    magicLinkHTML(link),
		Text:    magicLinkText(link),
	})
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
func magicLinkHTML(link string) string {
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
                <h1 style="margin:0 0 12px;font-size:24px;line-height:1.25;color:#111;">Sign in to ELITE</h1>
                <p style="margin:0 0 28px;font-size:15px;line-height:1.5;color:#555;">
                  Tap the button below to sign in. The link works once and expires in 15 minutes.
                </p>
                <a href="` + safe + `" style="display:inline-block;background:#111;color:#fff;text-decoration:none;font-size:15px;font-weight:600;padding:14px 28px;border-radius:12px;">
                  Sign in →
                </a>
                <p style="margin:28px 0 0;font-size:13px;line-height:1.5;color:#888;">
                  Or paste this link into your browser:<br />
                  <span style="color:#555;word-break:break-all;">` + safe + `</span>
                </p>
                <p style="margin:24px 0 0;font-size:13px;line-height:1.5;color:#888;">
                  If you didn't request this, you can ignore this email.
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

func magicLinkText(link string) string {
	return "Sign in to ELITE\n\n" +
		"Open this link to sign in. It works once and expires in 15 minutes.\n\n" +
		link + "\n\n" +
		"If you didn't request this, you can ignore this email.\n"
}

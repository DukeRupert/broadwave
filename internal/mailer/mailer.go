package mailer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const postmarkAPI = "https://api.postmarkapp.com/email"

type Mailer struct {
	serverToken   string
	messageStream string
	httpClient    *http.Client
}

func New(serverToken, messageStream string) *Mailer {
	return &Mailer{
		serverToken:   serverToken,
		messageStream: messageStream,
		httpClient:    &http.Client{},
	}
}

type Email struct {
	From          string         `json:"From"`
	To            string         `json:"To"`
	Subject       string         `json:"Subject"`
	TextBody      string         `json:"TextBody,omitempty"`
	HtmlBody      string         `json:"HtmlBody,omitempty"`
	MessageStream string         `json:"MessageStream,omitempty"`
	Headers       []EmailHeader  `json:"Headers,omitempty"`
}

type EmailHeader struct {
	Name  string `json:"Name"`
	Value string `json:"Value"`
}

type SendResponse struct {
	To          string `json:"To"`
	SubmittedAt string `json:"SubmittedAt"`
	MessageID   string `json:"MessageID"`
	ErrorCode   int    `json:"ErrorCode"`
	Message     string `json:"Message"`
}

func (m *Mailer) Send(email Email) (*SendResponse, error) {
	if email.MessageStream == "" {
		email.MessageStream = m.messageStream
	}

	body, err := json.Marshal(email)
	if err != nil {
		return nil, fmt.Errorf("marshaling email: %w", err)
	}

	req, err := http.NewRequest("POST", postmarkAPI, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Postmark-Server-Token", m.serverToken)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result SendResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if result.ErrorCode != 0 {
		return &result, fmt.Errorf("postmark error %d: %s", result.ErrorCode, result.Message)
	}

	return &result, nil
}

func (m *Mailer) SendConfirmation(to, fromName, fromEmail, listName, confirmURL string) error {
	subject := fmt.Sprintf("Confirm your signup — %s", listName)
	textBody := fmt.Sprintf(`Hey,

You signed up for updates from %s.
Click below to confirm:

%s

If you didn't sign up, just ignore this email.

— %s
`, listName, confirmURL, fromName)

	_, err := m.Send(Email{
		From:     fmt.Sprintf("%s <%s>", fromName, fromEmail),
		To:       to,
		Subject:  subject,
		TextBody: textBody,
	})
	return err
}

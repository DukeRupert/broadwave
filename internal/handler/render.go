package handler

import (
	"fmt"
	"strings"
)

func renderMessageBody(body, name, email, unsubscribeURL string) string {
	if name == "" {
		name = "there"
	}
	s := strings.ReplaceAll(body, "{{name}}", name)
	s = strings.ReplaceAll(s, "{{email}}", email)
	s = strings.ReplaceAll(s, "{{unsubscribe_url}}", unsubscribeURL)
	return s
}

func buildCANSPAMFooter(fromName, physicalAddress, unsubscribeURL string) string {
	return fmt.Sprintf(`
---
%s
%s

Unsubscribe: %s
`, fromName, physicalAddress, unsubscribeURL)
}

func buildCANSPAMFooterHTML(fromName, physicalAddress, unsubscribeURL string) string {
	return fmt.Sprintf(`<div style="margin-top:32px;padding-top:16px;border-top:1px solid #e5e7eb;font-size:12px;color:#6b7280;text-align:center;">
  <p>%s<br>%s</p>
  <p><a href="%s" style="color:#6b7280;">Unsubscribe</a></p>
</div>`, fromName, physicalAddress, unsubscribeURL)
}

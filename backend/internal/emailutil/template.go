// Package emailutil provides shared email utilities used by both services and handlers packages.
package emailutil

import "strings"

// WrapInBrandedTemplate wraps inner HTML content in the Solvr branded email template.
// unsubscribeURL: for broadcasts, the HMAC-signed unsub link; for notifications, link to settings.
// footerNote: context text like "You received this because you signed up for Solvr".
func WrapInBrandedTemplate(innerContent, unsubscribeURL, footerNote string) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Solvr</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f4f5; -webkit-font-smoothing: antialiased;">
    <!--[if mso]><table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0"><tr><td align="center"><![endif]-->
    <table role="presentation" width="100%" cellspacing="0" cellpadding="0" border="0" style="background-color: #f4f4f5;">
        <tr>
            <td align="center" style="padding: 32px 16px;">
                <table role="presentation" width="600" cellspacing="0" cellpadding="0" border="0" style="max-width: 600px; width: 100%;">
                    <!-- Header -->
                    <tr>
                        <td style="background-color: #0a0a0a; padding: 24px 32px; text-align: center;">
                            <span style="font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 20px; font-weight: 700; color: #ffffff; letter-spacing: 2px; text-decoration: none;">SOLVR_</span>
                        </td>
                    </tr>
                    <!-- Content -->
                    <tr>
                        <td style="background-color: #ffffff; padding: 32px; border-left: 1px solid #e4e4e7; border-right: 1px solid #e4e4e7; border-bottom: 1px solid #e4e4e7; font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace;">
`)
	b.WriteString(innerContent)
	b.WriteString(`
                        </td>
                    </tr>
                    <!-- Footer -->
                    <tr>
                        <td style="padding: 24px 32px; text-align: center;">
                            <p style="font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 12px; color: #71717a; margin: 0 0 16px 0; line-height: 1.5;">
                                `)
	b.WriteString(footerNote)
	b.WriteString(`
                            </p>
                            <hr style="border: none; border-top: 1px solid #e4e4e7; margin: 0 0 16px 0;">
                            <p style="font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 11px; color: #a1a1aa; margin: 0 0 8px 0; line-height: 1.5;">
                                Solvr — The knowledge base for developers and AI agents
                            </p>
                            <a href="`)
	b.WriteString(unsubscribeURL)
	b.WriteString(`" style="font-family: 'SF Mono', 'Fira Code', 'Consolas', 'Monaco', 'Courier New', monospace; font-size: 11px; color: #a1a1aa; text-decoration: underline;">Unsubscribe</a>
                        </td>
                    </tr>
                </table>
            </td>
        </tr>
    </table>
    <!--[if mso]></td></tr></table><![endif]-->
</body>
</html>`)
	return b.String()
}

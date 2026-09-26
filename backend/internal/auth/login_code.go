package auth

import "time"

// LoginCodeTTL is how long a one-time OAuth login code redeems. The browser lands on the
// frontend callback page with the code in the URL and exchanges it within a moment, so the life
// is kept this short: a code that leaks into a log, browser history or an analytics page view is
// useless almost at once, and useless immediately after the exchange.
const LoginCodeTTL = 60 * time.Second

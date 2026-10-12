# Security

_Last updated October 2026_

This page describes how RetroTide protects what your team writes, and how to tell us about a vulnerability.

## Hidden cards stay hidden

Every new board starts with its cards hidden, unless whoever creates it chooses to show them. While cards are hidden, the server sends each participant their own cards in full and, for everyone else's, only a count per column. Nothing else about another person's hidden card is sent: no text, author, votes, comments or timestamps. The blurred cards you see are empty placeholders, so there is nothing to uncover in the browser's developer tools.

## No facilitator peek

No role can read a hidden card before the reveal. Facilitators, team leads and organisation owners see their own cards and the counts, exactly like everyone else, and no board setting changes that.

## Anonymous boards

An anonymous board never shows who wrote a card, to anyone, facilitators included. RetroTide keeps the link between you and your cards only while the retro is open, so you can edit them. The link is deleted when the retro ends, or after 14 days without activity if nobody ends it, and backups that still hold it are deleted within 6 months. After that, neither who wrote a card nor who voted for it exists anywhere except in those backups.

Anonymous boards also send no card dates, and the order of cards is shuffled when they are revealed, so neither gives away who wrote what.

## Encryption in transit

Every connection to RetroTide is encrypted. Cloudflare terminates the connection and passes it on to our servers over a second encrypted connection, and our servers accept traffic from Cloudflare only. Browsers are told to use only encrypted connections to RetroTide.

Our cookies are sent only over encrypted connections, cannot be read by scripts on the page, and are tied to RetroTide's own address.

## Passwords

- Account passwords are hashed with bcrypt. We never store the password itself.
- Board passwords are hashed with bcrypt too.
- Signing in takes the same work whether or not an account exists, so the sign-in form does not reveal who has an account.
- Sign-in, sign-up, password reset and board passwords are rate limited.

## Sign in with Google

Google sign-in uses the standard protections against a sign-in being hijacked or replayed. We never keep a Google token.

## Logs

Our logs contain no board content. Requests for board pages are logged without the board's link or the visitor's IP address. Logs are kept for 30 days.

## Backups

We take nightly offsite backups, encrypted before they leave our server, and keep 7 daily, 4 weekly and 6 monthly copies. Nothing older than 6 months is kept.

## Reporting a vulnerability

If you think you have found a security problem in RetroTide, email hello@retrotide.app with the details and the steps to reproduce it. Please give us a reasonable time to fix it before you share it publicly.

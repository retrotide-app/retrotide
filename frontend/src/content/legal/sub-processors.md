# Sub-processors

_Last updated October 2026_

These are the companies that process personal data on our behalf to run RetroTide. Each one handles only what it needs for the job listed.

| Sub-processor | What it does | Where |
| --- | --- | --- |
| Our dedicated server host | Rents us the dedicated server that runs RetroTide and its database. | Where the server is located |
| Cloudflare | DNS, encrypted connections, the firewall and traffic statistics in front of RetroTide, and bot checks on sign-up and sign-in forms when they are switched on. | Cloudflare's global network |
| Cloudflare R2 | Holds the nightly offsite backups, encrypted before they leave our server and kept for up to 6 months. | EU |
| Cloudflare Email Sending | Sends account emails: verification, password reset, invitations and notifications. | Cloudflare's global network |
| Stripe | Payments, once paid plans go on sale. Not used yet. | Not used yet |

## Why Google is not on this list

If you choose Sign in with Google, Google receives your sign-in. Google is not our sub-processor: it is an independent controller. It is the identity provider you choose to sign in with, and it processes that sign-in under its own terms, not on our behalf. It processes no customer data for us and never receives board content. The [privacy notice](/legal/privacy) names Google and explains what we receive from it and what we keep.

## Changes

We will update this page before we add or replace a sub-processor. Questions go to hello@retrotide.app.

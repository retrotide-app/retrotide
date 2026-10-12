# Privacy notice

_Last updated October 2026_

This notice explains what personal data RetroTide holds, why, for how long, and what you can do about it.

## Who we are

RetroTide ("we") is run from the United Kingdom. You can reach us about anything in this notice at hello@retrotide.app.

## Our two roles

- **Controller.** We decide how your account, billing and marketing data is used, so we are its controller.
- **Processor.** Board content and guest data belong to the organisation that runs the board. We process them on that organisation's behalf and follow its instructions. If you have a question about what is on a board, ask the organisation that runs it first.

## Why we are allowed to use your data

- **To run the service you signed up for** (contract): your account, your organisations and teams, and the boards you take part in.
- **To keep RetroTide secure and free of abuse** (legitimate interests): IP addresses, rate limits, logs and error reports.
- **When the law requires it** (legal obligation): for example, answering a lawful request from the authorities.

## What we collect and why

### Your account

- Your email address, so you can sign in and we can send you account emails such as verification, password reset, invitations and notifications.
- Your display name, which other people see on boards and in your organisation.
- A hash of your password, made with bcrypt. We never store the password itself. An account created through Google has no password until you set one.
- Whether your email address is verified.
- The IP address you last used, for security. We clear it 30 days after it was recorded.

### Organisations and teams

- The names of your organisations and teams, who belongs to them and the role each person holds.
- A record of administrative actions, such as role changes and invitations, in an audit log. The audit log never records card, vote or comment writes. Any IP address in it is cleared after 90 days.

### Boards

- Board titles and descriptions, columns, cards, comments, votes, action items (with their assignee and due date) and mentions.
- Nothing you type is filtered. Facilitators can remove participants and cards.

### Guests

- The name a guest chooses, or "Anonymous".
- A guest device cookie, so a guest can come back to a board they joined by link. We store no IP address for guests.

### Logs and error reports

- Server logs, kept for 30 days. They never contain board content, and requests for board pages are logged without the board's link or your IP address.
- Error reports from your browser, kept for 30 days. A report is shortened before it is stored, but it can still contain some text from a card.

### Payments

Paid plans are not on sale yet. When they are, payments will be handled by Stripe, and we will update this notice before then.

### Analytics

We use no analytics that rely on cookies. Our usage numbers come from counters on our own servers and from Cloudflare's traffic statistics.

## Hidden cards and anonymous boards

While a board hides its cards, other participants' card text is never sent to your browser, only a count per column. No role can read a hidden card before the reveal, and that includes facilitators, team leads and organisation owners.

On an anonymous board: Anonymous to everyone on the board, including facilitators. RetroTide keeps a link between you and your cards only while the retro is open, so you can edit them. The link is deleted when the retro ends, or after 14 days without activity if nobody ends it, and backups that still hold it are deleted within 6 months.

## Sign in with Google

You can choose to sign in with Google. Google is a recipient of your data and an independent controller: it is the identity provider you choose to sign in with, and it processes that sign-in under its own terms, not on our behalf. Google never receives board content.

When you sign in, Google sends us:

- your Google account ID;
- your email address, and whether Google has verified it;
- your Workspace domain, if you have one;
- your name;
- your profile picture URL;
- for some accounts, your locale.

We keep:

- your Google account ID;
- the email address at the time you linked Google;
- for an account created through Google, your name as your starting display name, which you can change.

We never keep a Google token, and we discard everything else at once.

Your self-export includes each linked sign-in identity. Unlinking Google from your account settings, or deleting your account, deletes the link.

## Your data, your copy, your choice

- **Self-export.** The "Your data" section of your account page downloads a JSON file with your account, each linked sign-in identity (its provider, its subject, the address at the time of linking and the date it was linked), your organisation memberships with their roles and teams, and the cards, comments and action items still linked to you. Cards and comments on an anonymous board stop being linked to you once the retro ends, so they are not in the export after that.
- **Deletion.** You can delete your account from your account page. We refuse while you are the only owner of an organisation, until you transfer ownership or delete the organisation.

## How long we keep things

| What | How long |
| --- | --- |
| Logs and browser error reports | 30 days |
| Backups | Up to 6 months |
| Your last IP address | 30 days |
| IP addresses in the audit log | 90 days |
| Guest devices | 90 days |
| Linked Google identities | Until you unlink Google or delete your account |
| An unfinished Google sign-in | 10 minutes, cleared within the hour |
| Deleted boards and organisations | 30 days, so they can be restored, then purged |
| Free organisations with no member sign-in | Deleted after 18 months, with warnings 30 days and 7 days before |
| Anonymous authorship links | Until the retro ends or after 14 idle days, and gone from backups within 6 months |

## Where your data is stored

RetroTide runs on a dedicated server, and its encrypted backups are kept in Cloudflare R2 in the EU. The companies that help us run RetroTide are listed on the [sub-processors page](/legal/sub-processors).

## Cookies

We use strictly necessary cookies only. The [cookie statement](/legal/cookies) lists them.

## Your rights

Under UK data protection law you can ask us to:

- give you a copy of your data;
- correct data that is wrong;
- delete your data;
- restrict or object to how we use it;
- give you your data in a portable format.

Email hello@retrotide.app to make a request. For board content, which we process for an organisation, we will pass your request to that organisation.

You can also complain to the Information Commissioner's Office (ICO) at ico.org.uk.

## Changes to this notice

If we change this notice, we will update this page before the change takes effect.

## Contact

Email hello@retrotide.app.

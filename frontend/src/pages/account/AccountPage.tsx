import { useState } from "react";
import { PageHeader } from "../../components/PageHeader/PageHeader";
import { SettingsGroup } from "../../components/Settings/SettingsGroup";
import { useAuth } from "../../hooks/useAuth";
import { usePageTitle } from "../../hooks/usePageTitle";
import { DeleteAccountSection } from "./DeleteAccountSection";
import { EmailSection } from "./EmailSection";
import { LeaveOrganisationSection } from "./LeaveOrganisationSection";
import { MentionsSection } from "./MentionsSection";
import { PasswordSection } from "./PasswordSection";
import { ProfileSection } from "./ProfileSection";
import { SignInMethodsSection } from "./SignInMethodsSection";
import { SignOutOtherDevicesSection } from "./SignOutOtherDevicesSection";
import { TwoFactorSection } from "./TwoFactorSection";
import { YourDataSection } from "./YourDataSection";
import styles from "./AccountPage.module.css";

export function AccountPage() {
    const { user } = useAuth();
    const [requestedEmail, setRequestedEmail] = useState<string | null>(null);

    usePageTitle("Account");

    if (user === null) {
        return null;
    }

    const pendingEmail =
        requestedEmail !== null && requestedEmail.toLowerCase() !== user.email.toLowerCase() ? requestedEmail : null;

    return (
        <section className={styles.page} aria-labelledby="account-title">
            <PageHeader
                titleId="account-title"
                title="Account"
                description="Your name, email address, password, the ways you sign in, where you were mentioned and a copy of your data."
            />
            <SettingsGroup>
                <ProfileSection user={user} />
                <MentionsSection />
                <EmailSection user={user} pendingEmail={pendingEmail} onChangeRequested={setRequestedEmail} />
                <PasswordSection user={user} onPasswordChanged={() => setRequestedEmail(null)} />
                <TwoFactorSection user={user} />
                <SignInMethodsSection user={user} />
                <SignOutOtherDevicesSection user={user} onSignedOut={() => setRequestedEmail(null)} />
                <YourDataSection />
                <LeaveOrganisationSection />
                <DeleteAccountSection user={user} />
            </SettingsGroup>
        </section>
    );
}

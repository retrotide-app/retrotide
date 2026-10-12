import { Banner } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { SettingsRow } from "../../components/Settings/SettingsRow";
import { useSignOutOtherDevices } from "../../hooks/mutations/auth";
import type { SessionUser } from "../../types/app";
import styles from "./AccountSection.module.css";
import { IdentityList } from "./IdentityList";

interface SignOutOtherDevicesSectionProps {
    user: SessionUser;
    onSignedOut: () => void;
}

export function SignOutOtherDevicesSection({ user, onSignedOut }: SignOutOtherDevicesSectionProps) {
    const signOut = useSignOutOtherDevices();

    return (
        <SettingsRow
            title="Other devices"
            headingLevel={2}
            description="Sign out on every other browser and device. This also cancels a change of email address you have not confirmed."
        >
            {signOut.error !== null && <ErrorBanner message={signOut.error.message} takeFocus />}
            <div className={styles.actions}>
                <Button
                    type="button"
                    disabled={signOut.isPending}
                    onClick={() => {
                        signOut.mutate(undefined, { onSuccess: onSignedOut });
                    }}
                >
                    Sign out other devices
                </Button>
            </div>
            {signOut.isSuccess && (
                <>
                    <Banner tone="success" role="status">
                        You are signed out on every other device.
                    </Banner>
                    <h3 className={styles.subtitle}>Check your sign-in methods</h3>
                    <p className={styles.text}>Remove any you do not recognise.</p>
                    <IdentityList hasPassword={user.has_password} />
                </>
            )}
        </SettingsRow>
    );
}

import { Panel } from "../../components/Panel/Panel";
import { SetPasswordForm } from "../../components/SetPasswordForm/SetPasswordForm";

interface SetPasswordOfferProps {
    email: string;
}

export function SetPasswordOffer({ email }: SetPasswordOfferProps) {
    const description = (
        <>
            Your account has no password. We will email a link to <bdi>{email}</bdi> that lets you set one, so you can
            still sign in if you lose access to the account you signed up with.
        </>
    );

    return (
        <Panel title="Set a password" headingLevel={2} description={description}>
            <SetPasswordForm email={email} variant="secondary" />
        </Panel>
    );
}

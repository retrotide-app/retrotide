import { type SubmitEvent, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { Button } from "../../components/Button/Button";
import { ButtonLink } from "../../components/Button/ButtonLink";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../components/FormField/TextField";
import { AuthLayout } from "../../components/layout/AuthLayout/AuthLayout";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { TextLink } from "../../components/TextLink/TextLink";
import { ToggleSwitch } from "../../components/ToggleSwitch/ToggleSwitch";
import { joinBoardPath } from "../../domain/board/paths";
import {
    GUEST_NAME_MAX_LENGTH,
    type JoinDraft,
    joinedBoardPath,
    joinPasswordRefusal,
    type JoinProblems,
    joinProblems,
    joinRefusal,
    joinRequest,
    needsAccount,
    rejoinState,
} from "../../domain/board/shareLink";
import { returnState } from "../../domain/returnTo";
import { useJoinBoardByLink } from "../../hooks/mutations/shareLinks";
import { useJoinPreview } from "../../hooks/queries/shareLinks";
import { useAuth } from "../../hooks/useAuth";
import { useCurrentOrg } from "../../hooks/useCurrentOrg";
import { usePageTitle } from "../../hooks/usePageTitle";
import type { JoinPreviewResponse } from "../../types/api";
import styles from "./JoinBoardPage.module.css";

interface SignInOfferProps {
    slug: string;
}

interface JoinFormProps {
    slug: string;
    preview: JoinPreviewResponse;
}

const PAGE_TITLE = "Join a retrospective";

const ACCOUNT_NEEDED = "This board is only for people with an account. Sign in to join it.";

const CANNOT_JOIN = "You cannot join this board with this link.";

const EMPTY_DRAFT: JoinDraft = { name: "", anonymous: false, password: "" };

function SignInOffer({ slug }: SignInOfferProps) {
    return (
        <ButtonLink to="/login" state={returnState(joinBoardPath(slug), "")} variant="primary" className={styles.wide}>
            Sign in to join
        </ButtonLink>
    );
}

function JoinForm({ slug, preview }: JoinFormProps) {
    const { user } = useAuth();
    const { organisations } = useCurrentOrg();
    const join = useJoinBoardByLink(slug);
    const navigate = useNavigate();
    const [draft, setDraft] = useState<JoinDraft>(EMPTY_DRAFT);
    const [problems, setProblems] = useState<JoinProblems>({});

    const asGuest = user === null;
    const passwordError = problems.password ?? joinPasswordRefusal(join.error);
    const refusal = joinRefusal(join.error);
    const unexplained = join.error !== null && refusal === null && joinPasswordRefusal(join.error) === null;
    const offerSignIn = asGuest && needsAccount(join.error);

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        if (join.isPending) {
            return;
        }

        const found = joinProblems(draft, asGuest, preview.requires_password);
        setProblems(found);

        if (Object.keys(found).length > 0) {
            join.reset();
            return;
        }

        join.mutate(joinRequest(draft, asGuest, preview.requires_password), {
            onSuccess: joined => {
                navigate(joinedBoardPath(joined, organisations), { replace: true, state: rejoinState(slug) });
            },
        });
    }

    return (
        <form className={styles.form} onSubmit={handleSubmit} noValidate>
            {refusal !== null && <ErrorBanner message={refusal} takeFocus />}
            {unexplained && join.error !== null && <ErrorBanner message={join.error.message} takeFocus />}
            {offerSignIn && <SignInOffer slug={slug} />}
            {user !== null && (
                <p className={styles.lead}>
                    You join as <bdi>{user.display_name}</bdi>.
                </p>
            )}
            {asGuest && (
                <ToggleSwitch
                    label="Join as Anonymous"
                    description="Everyone on the board sees you as Anonymous instead of your name."
                    enabled={draft.anonymous}
                    onChange={anonymous => setDraft(current => ({ ...current, anonymous }))}
                />
            )}
            {asGuest && !draft.anonymous && (
                <TextField
                    label="Your name"
                    name="display-name"
                    autoComplete="nickname"
                    maxLength={GUEST_NAME_MAX_LENGTH}
                    hint="Shown beside your cards and in the list of people on the board."
                    value={draft.name}
                    onChange={event => {
                        const name = event.target.value;
                        setDraft(current => ({ ...current, name }));
                    }}
                    error={problems.name}
                />
            )}
            {preview.requires_password && (
                <TextField
                    label="Board password"
                    type="password"
                    name="board-password"
                    autoComplete="off"
                    value={draft.password}
                    onChange={event => {
                        const password = event.target.value;
                        setDraft(current => ({ ...current, password }));
                    }}
                    error={passwordError}
                />
            )}
            <Button type="submit" variant="primary" className={styles.wide} disabled={join.isPending}>
                Join board
            </Button>
        </form>
    );
}

export function JoinBoardPage() {
    usePageTitle(PAGE_TITLE);

    const { slug = "" } = useParams();
    const { user, loading } = useAuth();
    const preview = useJoinPreview(slug);

    const footer =
        user === null ? (
            <p>
                Have an account?{" "}
                <TextLink to="/login" state={returnState(joinBoardPath(slug), "")}>
                    Sign in
                </TextLink>
            </p>
        ) : undefined;

    if (preview.error !== null) {
        const refusal = joinRefusal(preview.error);

        return (
            <AuthLayout title={PAGE_TITLE} footer={footer}>
                <div className={styles.form}>
                    {refusal === null ? (
                        <LoadFailure message={preview.error.message} onRetry={preview.refetch} />
                    ) : (
                        <ErrorBanner message={refusal} />
                    )}
                    {user === null && needsAccount(preview.error) && <SignInOffer slug={slug} />}
                </div>
            </AuthLayout>
        );
    }

    const answer = preview.data;

    if (loading || answer === undefined) {
        return (
            <AuthLayout title={PAGE_TITLE}>
                <p className="loading">Loading...</p>
            </AuthLayout>
        );
    }

    if (!answer.can_join) {
        const accountNeeded = answer.requires_account && user === null;

        return (
            <AuthLayout title={PAGE_TITLE} footer={accountNeeded ? undefined : footer}>
                <div className={styles.form}>
                    <ErrorBanner message={accountNeeded ? ACCOUNT_NEEDED : CANNOT_JOIN} />
                    {accountNeeded && <SignInOffer slug={slug} />}
                </div>
            </AuthLayout>
        );
    }

    return (
        <AuthLayout title={PAGE_TITLE} footer={footer}>
            <p className={styles.lead}>Someone shared a retrospective with you. Join it to add your cards.</p>
            <JoinForm slug={slug} preview={answer} />
        </AuthLayout>
    );
}

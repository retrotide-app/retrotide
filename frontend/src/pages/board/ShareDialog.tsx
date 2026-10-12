import { toDataURL } from "qrcode";
import { type SubmitEvent, useEffect, useId, useState } from "react";
import { Banner, BannerButton } from "../../components/Banner/Banner";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { Input } from "../../components/Input/Input";
import { LoadFailure } from "../../components/LoadFailure/LoadFailure";
import { Modal } from "../../components/Modal/Modal";
import { ToggleGroup, type ToggleOption } from "../../components/ToggleGroup/ToggleGroup";
import { LINK_ACCESS_OPTIONS, shareLinkUrl } from "../../domain/board/shareLink";
import { BOARD_PASSWORD_REFUSALS, fieldRefusal } from "../../domain/refusals";
import { useRotateShareLink, useSetBoardPassword, useSetLinkAccess } from "../../hooks/mutations/shareLinks";
import { useBoardSharing } from "../../hooks/queries/shareLinks";
import { useCopyText } from "../../hooks/useCopyText";
import { pageOrigin } from "../../platform/location";
import type { BoardSharingResponse, LinkAccess } from "../../types/api";
import { errorMessage } from "../../utils/errorMessage";
import styles from "./ShareDialog.module.css";

interface ShareDialogProps {
    boardId: string;
    onClose: () => void;
}

interface SharingFormProps {
    boardId: string;
    sharing: BoardSharingResponse;
}

interface LinkSectionProps {
    sharing: BoardSharingResponse;
}

interface RotateSectionProps {
    boardId: string;
}

interface LinkQrProps {
    url: string;
}

type QrDrawing = { drawn: string; image: string } | { drawn: string; failure: string };

const ACCESS_TOGGLES: readonly ToggleOption<LinkAccess>[] = LINK_ACCESS_OPTIONS.map(option => ({
    value: option.value,
    label: option.label,
}));

const PASSWORD_MISSING = "Enter a password.";

const QR_SIZE = 160;

const QR_MARGIN = 1;

function LinkQr({ url }: LinkQrProps) {
    const [drawing, setDrawing] = useState<QrDrawing | null>(null);

    useEffect(() => {
        let active = true;

        toDataURL(url, { width: QR_SIZE, margin: QR_MARGIN })
            .then(image => {
                if (active) {
                    setDrawing({ drawn: url, image });
                }
            })
            .catch((failure: unknown) => {
                if (active) {
                    setDrawing({ drawn: url, failure: errorMessage(failure, "the QR code library failed") });
                }
            });

        return () => {
            active = false;
        };
    }, [url]);

    if (drawing === null || drawing.drawn !== url) {
        return <p className={styles.note}>Drawing the QR code...</p>;
    }

    if ("failure" in drawing) {
        return <ErrorBanner message={`The QR code could not be drawn (${drawing.failure}). Use the link instead.`} />;
    }

    return (
        <img
            className={styles.qr}
            src={drawing.image}
            alt="QR code for the board link"
            width={QR_SIZE}
            height={QR_SIZE}
        />
    );
}

function LinkSection({ sharing }: LinkSectionProps) {
    const headingId = useId();
    const copy = useCopyText();
    const url = shareLinkUrl(pageOrigin(), sharing.share_slug);

    return (
        <section aria-labelledby={headingId} className={styles.section}>
            <h4 id={headingId} className={styles.heading}>
                Board link
            </h4>
            <div className={styles.linkRow}>
                <div className={styles.linkField}>
                    <Input
                        aria-label="Link to the board"
                        fullWidth
                        value={url}
                        readOnly
                        onFocus={e => e.target.select()}
                    />
                </div>
                <Button type="button" variant="primary" onClick={() => copy.copy(url)}>
                    Copy link
                </Button>
            </div>
            {sharing.link_access === "off" && (
                <p className={styles.note}>The link is switched off, so nobody can join with it yet.</p>
            )}
            <LinkQr url={url} />
            {copy.copied && (
                <Banner tone="success" role="status">
                    Link copied.
                </Banner>
            )}
            {copy.error !== null && <ErrorBanner message={copy.error.message} takeFocus />}
        </section>
    );
}

function AccessSection({ boardId, sharing }: SharingFormProps) {
    const headingId = useId();
    const access = useSetLinkAccess(boardId);
    const chosen = LINK_ACCESS_OPTIONS.find(option => option.value === sharing.link_access);

    return (
        <section aria-labelledby={headingId} className={styles.section}>
            <h4 id={headingId} className={styles.heading}>
                Who can join with the link
            </h4>
            <ToggleGroup
                label="Who can join with the link"
                options={ACCESS_TOGGLES}
                value={sharing.link_access}
                onChange={linkAccess => {
                    if (access.isPending || linkAccess === sharing.link_access) {
                        return;
                    }

                    access.mutate(linkAccess);
                }}
            />
            {chosen !== undefined && <p className={styles.note}>{chosen.description}</p>}
            {access.error !== null && <ErrorBanner message={access.error.message} takeFocus />}
        </section>
    );
}

function PasswordSection({ boardId, sharing }: SharingFormProps) {
    const headingId = useId();
    const password = useSetBoardPassword(boardId);
    const [draft, setDraft] = useState("");
    const [problem, setProblem] = useState<string | null>(null);

    const fieldError = problem ?? fieldRefusal(password.error, BOARD_PASSWORD_REFUSALS);
    const otherError = fieldRefusal(password.error, BOARD_PASSWORD_REFUSALS) === null ? password.error : null;

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        if (password.isPending) {
            return;
        }

        if (draft === "") {
            setProblem(PASSWORD_MISSING);
            password.reset();
            return;
        }

        setProblem(null);
        password.mutate(draft, {
            onSuccess: () => {
                setDraft("");
            },
        });
    }

    function handleRemove() {
        setProblem(null);
        setDraft("");
        password.mutate(null);
    }

    return (
        <section aria-labelledby={headingId} className={styles.section}>
            <h4 id={headingId} className={styles.heading}>
                Password protection
            </h4>
            <p className={styles.note}>
                {sharing.has_password
                    ? "The board has a password. Everyone who joins with the link must enter it."
                    : "Add a password so only people who know it can join with the link."}{" "}
                Setting, changing or removing it makes everyone who joined with the link join again.
            </p>
            <form className={styles.passwordRow} onSubmit={handleSubmit} aria-label="Board password" noValidate>
                <div className={styles.linkField}>
                    <Input
                        aria-label={sharing.has_password ? "New password" : "Password"}
                        aria-invalid={fieldError !== null}
                        fullWidth
                        type="password"
                        name="board-password"
                        autoComplete="new-password"
                        placeholder={sharing.has_password ? "New password" : "Password"}
                        value={draft}
                        onChange={event => setDraft(event.target.value)}
                    />
                </div>
                <div className={styles.buttons}>
                    <Button type="submit" variant="primary" disabled={password.isPending}>
                        {sharing.has_password ? "Change password" : "Set password"}
                    </Button>
                    {sharing.has_password && (
                        <Button type="button" variant="danger" disabled={password.isPending} onClick={handleRemove}>
                            Remove password
                        </Button>
                    )}
                </div>
            </form>
            {password.isSuccess && (
                <Banner tone="success" role="status">
                    {sharing.has_password ? "Password saved." : "Password removed."}
                </Banner>
            )}
            {fieldError !== null && <ErrorBanner message={fieldError} takeFocus />}
            {otherError !== null && <ErrorBanner message={otherError.message} takeFocus />}
        </section>
    );
}

function RotateSection({ boardId }: RotateSectionProps) {
    const headingId = useId();
    const rotate = useRotateShareLink(boardId);
    const [confirming, setConfirming] = useState(false);

    function handleRotate() {
        rotate.mutate(undefined, {
            onSettled: () => {
                setConfirming(false);
            },
        });
    }

    const confirmActions = (
        <>
            <BannerButton onClick={handleRotate} disabled={rotate.isPending}>
                Make a new link
            </BannerButton>
            <BannerButton onClick={() => setConfirming(false)} disabled={rotate.isPending}>
                Keep this link
            </BannerButton>
        </>
    );

    return (
        <section aria-labelledby={headingId} className={styles.section}>
            <h4 id={headingId} className={styles.heading}>
                New link
            </h4>
            <p className={styles.note}>Make a new link when the old one reached people who should not have it.</p>
            {confirming ? (
                <Banner tone="warning" role="alert" actions={confirmActions}>
                    The current link stops working at once, and everyone who joined with it must join again with the new
                    one.
                </Banner>
            ) : (
                <div>
                    <Button
                        type="button"
                        onClick={() => {
                            rotate.reset();
                            setConfirming(true);
                        }}
                    >
                        Make a new link
                    </Button>
                </div>
            )}
            {rotate.isSuccess && (
                <Banner tone="success" role="status">
                    New link made. Share it with the people you want on the board.
                </Banner>
            )}
            {rotate.error !== null && <ErrorBanner message={rotate.error.message} takeFocus />}
        </section>
    );
}

export function ShareDialog({ boardId, onClose }: ShareDialogProps) {
    const sharing = useBoardSharing(boardId);
    const current = sharing.data;

    return (
        <Modal isOpen onClose={onClose} title="Share this board">
            <div className={styles.dialog}>
                {sharing.error !== null && <LoadFailure message={sharing.error.message} onRetry={sharing.refetch} />}
                {current === undefined && sharing.error === null && <p className="loading">Loading sharing...</p>}
                {current !== undefined && (
                    <>
                        <LinkSection sharing={current} />
                        <AccessSection boardId={boardId} sharing={current} />
                        <PasswordSection boardId={boardId} sharing={current} />
                        <RotateSection boardId={boardId} />
                    </>
                )}
                <div className={styles.actions}>
                    <Button type="button" variant="secondary" onClick={onClose}>
                        Done
                    </Button>
                </div>
            </div>
        </Modal>
    );
}

import { type SubmitEvent, useState } from "react";
import imageStyles from "../../components/board/images/Images.module.css";
import { Button } from "../../components/Button/Button";
import { ErrorBanner } from "../../components/ErrorBanner/ErrorBanner";
import { TextField } from "../../components/FormField/TextField";
import { Panel } from "../../components/Panel/Panel";
import { IMAGE_ACCEPT, IMAGE_HINT, imageRefusal } from "../../domain/board/images";
import { useSetBoardBackground, useUploadMedia } from "../../hooks/mutations/media";
import { useImagePreview } from "../../hooks/useImagePreview";
import type { BoardSnapshot } from "../../types/api";
import sectionStyles from "../org/shared/OrgSection.module.css";

const NO_FILE_PROBLEM = "Choose an image to upload.";

export function BoardBackgroundSection({ snapshot }: { snapshot: BoardSnapshot }) {
    const upload = useUploadMedia(snapshot.board_id);
    const setBackground = useSetBoardBackground(snapshot.board_id);
    const [file, setFile] = useState<File | null>(null);
    const [problem, setProblem] = useState<string | null>(null);
    const picked = useImagePreview(file);

    const current = snapshot.board.background_path;
    const busy = upload.isPending || setBackground.isPending;
    const failure = upload.error ?? setBackground.error;
    const message = problem ?? (failure === null ? null : imageRefusal(failure));
    const shown = picked ?? current ?? null;

    function handlePick(files: FileList | null) {
        setProblem(null);
        setFile(files?.item(0) ?? null);
    }

    function handleSubmit(event: SubmitEvent<HTMLFormElement>) {
        event.preventDefault();

        if (busy) {
            return;
        }

        if (file === null) {
            setProblem(NO_FILE_PROBLEM);
            return;
        }

        setProblem(null);
        upload.mutate(
            { kind: "background", file },
            {
                onSuccess: media => setBackground.mutate({ imageId: media.id }, { onSuccess: () => setFile(null) }),
            },
        );
    }

    function handleRemove() {
        setProblem(null);
        setFile(null);
        setBackground.mutate({ imageId: null });
    }

    return (
        <Panel
            title="Background"
            headingLevel={2}
            description="An image shown behind the board's columns for everyone."
        >
            <form className={sectionStyles.form} onSubmit={handleSubmit} aria-label="Background" noValidate>
                {message !== null && <ErrorBanner message={message} takeFocus />}
                <TextField
                    key={current ?? "none"}
                    label="Background image"
                    hint={IMAGE_HINT}
                    type="file"
                    accept={IMAGE_ACCEPT}
                    disabled={busy}
                    onChange={event => handlePick(event.target.files)}
                />
                <div className={imageStyles.preview}>
                    {shown === null ? (
                        <p className={imageStyles.empty}>No background yet.</p>
                    ) : (
                        <img src={shown} alt="Preview of the board's background" className={imageStyles.previewImage} />
                    )}
                </div>
                <div className={sectionStyles.actions}>
                    <Button type="submit" variant="primary" disabled={busy}>
                        Upload background
                    </Button>
                    {current !== undefined && (
                        <Button type="button" variant="danger" disabled={busy} onClick={handleRemove}>
                            Remove background
                        </Button>
                    )}
                </div>
            </form>
        </Panel>
    );
}

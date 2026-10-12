import { useState } from "react";
import { IMAGE_ACCEPT, IMAGE_HINT, imageRefusal } from "../../../domain/board/images";
import { useSetCardImage, useUploadMedia } from "../../../hooks/mutations/media";
import { useImagePreview } from "../../../hooks/useImagePreview";
import type { BoardSnapshot, CardView } from "../../../types/api";
import { Button } from "../../Button/Button";
import { ErrorBanner } from "../../ErrorBanner/ErrorBanner";
import { TextField } from "../../FormField/TextField";
import { FormDialog } from "../../FormDialog/FormDialog";
import styles from "./Images.module.css";

const NO_FILE_PROBLEM = "Choose an image to upload.";

export function CardImageDialog({
    snapshot,
    card,
    onClose,
}: {
    snapshot: BoardSnapshot;
    card: CardView;
    onClose: () => void;
}) {
    const upload = useUploadMedia(snapshot.board_id);
    const setImage = useSetCardImage(snapshot.board_id);
    const [file, setFile] = useState<File | null>(null);
    const [problem, setProblem] = useState<string | null>(null);
    const picked = useImagePreview(file);

    const busy = upload.isPending || setImage.isPending;
    const failure = upload.error ?? setImage.error;
    const message = problem ?? (failure === null ? null : imageRefusal(failure));
    const shown = picked ?? card.image_path ?? null;

    function handlePick(files: FileList | null) {
        setProblem(null);
        setFile(files?.item(0) ?? null);
    }

    function handleUpload() {
        if (file === null) {
            setProblem(NO_FILE_PROBLEM);
            return;
        }

        setProblem(null);
        upload.mutate(
            { kind: "card", file },
            { onSuccess: media => setImage.mutate({ cardId: card.id, imageId: media.id }, { onSuccess: onClose }) },
        );
    }

    function handleRemove() {
        setProblem(null);
        setImage.mutate({ cardId: card.id, imageId: null }, { onSuccess: onClose });
    }

    return (
        <FormDialog
            open
            title={card.image_path === undefined ? "Add an image to the card" : "Change the card's image"}
            submitLabel="Upload image"
            busy={busy}
            onSubmit={handleUpload}
            onCancel={onClose}
            extraActions={
                card.image_path !== undefined && (
                    <Button type="button" variant="danger" disabled={busy} onClick={handleRemove}>
                        Remove image
                    </Button>
                )
            }
        >
            {message !== null && <ErrorBanner message={message} takeFocus />}
            <TextField
                label="Image file"
                hint={IMAGE_HINT}
                type="file"
                accept={IMAGE_ACCEPT}
                disabled={busy}
                onChange={event => handlePick(event.target.files)}
            />
            <div className={styles.preview}>
                {shown === null ? (
                    <p className={styles.empty}>No image yet.</p>
                ) : (
                    <img src={shown} alt="Preview of the card's image" className={styles.previewImage} />
                )}
            </div>
        </FormDialog>
    );
}

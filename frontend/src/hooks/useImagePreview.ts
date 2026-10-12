import { useEffect, useState } from "react";

interface ReadPreview {
    file: File;
    url: string;
}

export function useImagePreview(file: File | null): string | null {
    const [preview, setPreview] = useState<ReadPreview | null>(null);

    useEffect(() => {
        if (file === null) {
            return;
        }

        const reader = new FileReader();

        function handleLoad() {
            if (typeof reader.result !== "string" || file === null) {
                return;
            }

            setPreview({ file, url: reader.result });
        }

        reader.addEventListener("load", handleLoad);
        reader.readAsDataURL(file);

        return () => {
            reader.removeEventListener("load", handleLoad);
            reader.abort();
        };
    }, [file]);

    if (preview === null || preview.file !== file) {
        return null;
    }

    return preview.url;
}

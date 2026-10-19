import { errorCode } from "../../utils/errorMessage";

export const IMAGE_ACCEPT = "image/png,image/jpeg,image/webp";

export const IMAGE_HINT = "A PNG, JPEG or WebP image. GIFs cannot be used.";

const BYTES_PER_MEGABYTE = 1024 * 1024;

const BYTES_PER_GIGABYTE = 1024 * BYTES_PER_MEGABYTE;

const STORAGE_LIMIT = "media_bytes";

const IMAGE_REFUSALS: Readonly<Record<string, string>> = {
    unsupported_image: "Only PNG, JPEG and WebP images can be used. GIFs and other files cannot.",
    image_dimensions: "That image is too large. It can have at most 40 megapixels.",
    images_unavailable: "Images cannot be processed right now. Try again in a few minutes.",
};

function upgradeField(error: Error, field: string): unknown {
    const body: unknown = Reflect.get(error, "body");

    if (body === null || typeof body !== "object") {
        return undefined;
    }

    return Reflect.get(body, field);
}

function allowedBytes(error: Error): number | null {
    const allowed = upgradeField(error, "allowed");

    return typeof allowed === "number" && allowed > 0 ? allowed : null;
}

function storageRefusal(allowed: number | null): string {
    if (allowed === null) {
        return "Your organisation has used all the image storage its plan allows.";
    }

    const gigabytes = Math.round((allowed / BYTES_PER_GIGABYTE) * 10) / 10;

    return `Your organisation has used all ${gigabytes} GB of image storage its plan allows.`;
}

function sizeRefusal(error: Error): string {
    const allowed = allowedBytes(error);

    if (upgradeField(error, "limit") === STORAGE_LIMIT) {
        return storageRefusal(allowed);
    }

    if (allowed === null) {
        return "That image is larger than your plan allows.";
    }

    const megabytes = Math.round((allowed / BYTES_PER_MEGABYTE) * 10) / 10;

    return `That image is larger than your plan allows. Images can be up to ${megabytes} MB.`;
}

export function imageRefusal(error: Error): string {
    const code = errorCode(error);

    if (code === "upgrade_required") {
        return sizeRefusal(error);
    }

    if (code !== null && Object.hasOwn(IMAGE_REFUSALS, code)) {
        return IMAGE_REFUSALS[code];
    }

    return error.message;
}

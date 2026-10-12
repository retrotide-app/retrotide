import { errorCode } from "../../utils/errorMessage";

export const IMAGE_ACCEPT = "image/png,image/jpeg,image/webp";

export const IMAGE_HINT = "A PNG, JPEG or WebP image. GIFs cannot be used.";

const BYTES_PER_MEGABYTE = 1024 * 1024;

const IMAGE_REFUSALS: Readonly<Record<string, string>> = {
    unsupported_image: "Only PNG, JPEG and WebP images can be used. GIFs and other files cannot.",
    image_dimensions: "That image is too large. It can have at most 40 megapixels.",
    images_unavailable: "Images cannot be processed right now. Try again in a few minutes.",
};

function allowedBytes(error: Error): number | null {
    const body: unknown = Reflect.get(error, "body");

    if (body === null || typeof body !== "object") {
        return null;
    }

    const allowed: unknown = Reflect.get(body, "allowed");

    return typeof allowed === "number" && allowed > 0 ? allowed : null;
}

function sizeRefusal(error: Error): string {
    const allowed = allowedBytes(error);

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

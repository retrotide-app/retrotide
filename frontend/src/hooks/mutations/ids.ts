import { UUID_BYTE_LENGTH, uuidV4FromBytes } from "../../utils/uuid";

export function newId(): string {
    const random = crypto.getRandomValues(new Uint8Array(UUID_BYTE_LENGTH));

    return uuidV4FromBytes(random);
}

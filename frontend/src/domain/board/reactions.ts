import type { ReactionKey } from "../../types/api";

export interface ReactionOption {
    key: ReactionKey;
    emoji: string;
    label: string;
}

const REACTION_ORDER: readonly ReactionKey[] = [
    "thumbs_up",
    "thumbs_down",
    "heart",
    "laugh",
    "wow",
    "sad",
    "angry",
    "party",
    "rocket",
    "fire",
    "clap",
    "thinking",
    "eyes",
    "hundred",
    "star",
];

const REACTION_DETAILS: Readonly<Record<ReactionKey, Omit<ReactionOption, "key">>> = {
    thumbs_up: { emoji: "👍", label: "Thumbs up" },
    thumbs_down: { emoji: "👎", label: "Thumbs down" },
    heart: { emoji: "❤️", label: "Love" },
    laugh: { emoji: "😂", label: "Laugh" },
    wow: { emoji: "😮", label: "Wow" },
    sad: { emoji: "😢", label: "Sad" },
    angry: { emoji: "😠", label: "Angry" },
    party: { emoji: "🎉", label: "Party" },
    rocket: { emoji: "🚀", label: "Rocket" },
    fire: { emoji: "🔥", label: "Fire" },
    clap: { emoji: "👏", label: "Clap" },
    thinking: { emoji: "🤔", label: "Thinking" },
    eyes: { emoji: "👀", label: "Eyes" },
    hundred: { emoji: "💯", label: "Hundred" },
    star: { emoji: "⭐", label: "Star" },
};

export function reactionOption(key: ReactionKey): ReactionOption {
    return { key, ...REACTION_DETAILS[key] };
}

export const REACTIONS: readonly ReactionOption[] = REACTION_ORDER.map(reactionOption);

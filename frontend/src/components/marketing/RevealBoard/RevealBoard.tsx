import { type CSSProperties, useState } from "react";
import { Button } from "../../Button/Button";
import styles from "./RevealBoard.module.css";

type NoteTone = "green" | "orange" | "purple";

interface DemoCard {
    id: string;
    text: string;
    mine: boolean;
    votes: number;
}

interface DemoColumn {
    id: string;
    title: string;
    tone: NoteTone;
    cards: DemoCard[];
}

const COLUMNS: DemoColumn[] = [
    {
        id: "went-well",
        title: "Went well",
        tone: "green",
        cards: [
            { id: "deploys", text: "Deploys were smooth all sprint", mine: true, votes: 3 },
            { id: "pairing", text: "Pairing on the search bug paid off", mine: false, votes: 2 },
        ],
    },
    {
        id: "to-improve",
        title: "To improve",
        tone: "orange",
        cards: [
            { id: "standups", text: "Stand-ups keep running over", mine: false, votes: 4 },
            { id: "criteria", text: "Too many tickets without acceptance criteria", mine: false, votes: 1 },
        ],
    },
    {
        id: "actions",
        title: "Actions",
        tone: "purple",
        cards: [
            { id: "timebox", text: "Timebox stand-ups to 15 minutes", mine: false, votes: 3 },
            { id: "template", text: "Add acceptance criteria to the ticket template", mine: false, votes: 2 },
        ],
    },
];

export function RevealBoard() {
    const [revealed, setRevealed] = useState(false);

    return (
        <figure className={styles.board} aria-label="Example retro board" data-revealed={revealed}>
            <div className={styles.toolbar}>
                <div>
                    <p className={styles.boardName}>Sprint 42 retro</p>
                    <p className={styles.status} aria-live="polite">
                        {revealed ? "All cards revealed" : "5 people writing, cards hidden"}
                    </p>
                </div>
                <Button
                    type="button"
                    variant={revealed ? "secondary" : "primary"}
                    size="small"
                    aria-pressed={revealed}
                    onClick={() => setRevealed(current => !current)}
                >
                    {revealed ? "Hide cards" : "Reveal cards"}
                </Button>
            </div>
            <div className={styles.columns}>
                {COLUMNS.map((column, columnIndex) => (
                    <div key={column.id} role="group" className={styles.column} aria-label={column.title}>
                        <p className={styles.columnTitle} aria-hidden="true">
                            {column.title}
                        </p>
                        <ul className={styles.cards}>
                            {column.cards.map((card, cardIndex) => {
                                const visible = revealed || card.mine;
                                const position = { "--column": columnIndex, "--card": cardIndex } as CSSProperties;

                                return (
                                    <li
                                        key={card.id}
                                        className={`${styles.note} ${styles[column.tone]}`}
                                        data-mine={card.mine}
                                        style={position}
                                    >
                                        {visible ? (
                                            <span className={styles.text}>{card.text}</span>
                                        ) : (
                                            <span className={styles.placeholder}>
                                                <span className="visually-hidden">Hidden card</span>
                                                <span className={styles.line} aria-hidden="true" />
                                                <span className={styles.lineShort} aria-hidden="true" />
                                            </span>
                                        )}
                                        {card.mine && !revealed ? (
                                            <span className={styles.meta}>Only you can see this</span>
                                        ) : null}
                                        {revealed ? (
                                            <span className={styles.meta}>
                                                {card.votes} {card.votes === 1 ? "vote" : "votes"}
                                            </span>
                                        ) : null}
                                    </li>
                                );
                            })}
                        </ul>
                    </div>
                ))}
            </div>
        </figure>
    );
}

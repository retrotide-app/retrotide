import { Component, type ReactNode } from "react";
import { Button } from "../Button/Button";
import { ButtonLink } from "../Button/ButtonLink";
import styles from "./RootErrorBoundary.module.css";

interface RootErrorBoundaryProps {
    children: ReactNode;
}

interface RootErrorBoundaryState {
    failed: boolean;
    detail: string;
}

const INTACT: RootErrorBoundaryState = { failed: false, detail: "" };

function detailOf(error: unknown): string {
    if (error instanceof Error) {
        return error.message === "" ? error.name : `${error.name}: ${error.message}`;
    }

    if (typeof error === "string") {
        return error;
    }

    return "";
}

export class RootErrorBoundary extends Component<RootErrorBoundaryProps, RootErrorBoundaryState> {
    state: RootErrorBoundaryState = INTACT;

    static getDerivedStateFromError(error: unknown): RootErrorBoundaryState {
        return { failed: true, detail: detailOf(error) };
    }

    render(): ReactNode {
        if (!this.state.failed) {
            return this.props.children;
        }

        return (
            <div className={styles.page}>
                <div className={styles.card}>
                    <h1 className={styles.title}>Something went wrong</h1>
                    <p className={styles.blurb}>
                        RetroTide hit a problem while drawing this page. Try again first, which keeps everything you
                        still have open. Reload the page only if it will not come back.
                    </p>
                    {this.state.detail !== "" && (
                        <p dir="auto" className={styles.detail}>
                            {this.state.detail}
                        </p>
                    )}
                    <div className={styles.actions}>
                        <Button
                            type="button"
                            variant="primary"
                            onClick={() => {
                                this.setState(INTACT);
                            }}
                        >
                            Try again
                        </Button>
                        <Button
                            type="button"
                            onClick={() => {
                                window.location.reload();
                            }}
                        >
                            Reload the page
                        </Button>
                        <ButtonLink href="/">Back to RetroTide</ButtonLink>
                    </div>
                </div>
            </div>
        );
    }
}

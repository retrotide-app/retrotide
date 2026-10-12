import { ButtonLink } from "../../components/Button/ButtonLink";
import { usePageTitle } from "../../hooks/usePageTitle";
import styles from "./NotFoundPage.module.css";

export function NotFoundPage() {
    usePageTitle("Page not found");

    return (
        <section className={styles.page}>
            <div className={styles.code}>404</div>
            <h1 className={styles.title}>Page not found</h1>
            <p className={styles.blurb}>
                The page you asked for does not exist. It may have moved, or the address may have been mistyped.
            </p>
            <ButtonLink to="/" variant="primary">
                Back to the home page
            </ButtonLink>
        </section>
    );
}

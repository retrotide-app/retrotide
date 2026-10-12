import styles from "./Images.module.css";

export function CardImage({ path, alt }: { path: string; alt: string }) {
    return <img src={path} alt={alt} className={styles.cardImage} loading="lazy" decoding="async" />;
}

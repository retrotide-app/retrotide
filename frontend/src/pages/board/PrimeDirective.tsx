import { Banner } from "../../components/Banner/Banner";

export function PrimeDirective() {
    return (
        <Banner tone="info" role="note" label="The Prime Directive">
            <strong>The Prime Directive:</strong> &ldquo;Regardless of what we discover, we understand and truly believe
            that everyone did the best job they could, given what they knew at the time, their skills and abilities, the
            resources available, and the situation at hand.&rdquo; (Norm Kerth)
        </Banner>
    );
}

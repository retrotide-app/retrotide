import { useLocation } from "react-router";
import { returnPathFrom } from "../domain/returnTo";

const HOME = "/";

export function useReturnPath(): string {
    const location = useLocation();

    return returnPathFrom(location.state) ?? HOME;
}

export interface ClientErrorReportRequest {
    name: string;
    message: string;
    stack: string | null;
    component_stack: string | null;
    source: string;
    route: string;
    version: string;
    sequence: number;
}

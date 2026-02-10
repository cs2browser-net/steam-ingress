export interface Filters {
    [key: string]: string | number | string[] | Filters;
}

export interface Rules {
    limit: number;
    filter: Filters;
}

export interface Config {
    steam_api_keys: string[];
    retrieval_filters: Rules[];
    filtered_asn: number[];
    expressions: Record<string, number>;
}
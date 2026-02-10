import { Parser } from "expr-eval";
import type { Config, Filters } from "../types/config.ts";

const parser = new Parser();

parser.functions.number = (x: any) => Number(x);
parser.functions.string = (x: any) => String(x);
parser.functions.startsWith = (x: any, y: any) => String(x).startsWith(String(y));
parser.functions.includes = (x: any, y: any) => String(x).includes(String(y));

function countInstructions(filters: Filters): number {
    let count = 0;
    for (const key in filters) {
        if (Array.isArray(filters[key])) count += (filters[key] as any[]).length;
        else count += 1;
    }
    return count;
}

export function buildFilters(filters: Filters): string {
    const filterStrings: string[] = [];

    for (const key in filters) {
        if (key == "nand" || key == "nor") filterStrings.push(`${key}\\${countInstructions(filters[key] as Filters)}${buildFilters(filters[key] as Filters)}`);
        else if (Array.isArray(filters[key])) {
            const elements: any[] = filters[key] as any[];
            for (const element of elements) {
                filterStrings.push(`${key}\\${element}`);
            }
        } else filterStrings.push(`${key}\\${filters[key]}`);
    }

    return "\\" + filterStrings.join("\\");
}

export function evaluateExpression(cfg: Config, expression: string, data: Record<string, any>): number {
    const expr = parser.parse(expression);
    return expr.evaluate(data) ? cfg.expressions[expression] : 0;
}
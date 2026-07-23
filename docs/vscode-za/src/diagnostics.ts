import {
    Diagnostic,
    DiagnosticCollection,
    DiagnosticSeverity,
    languages,
    Range,
    TextDocument,
    workspace,
} from 'vscode';

let diagnosticCollection: DiagnosticCollection;

const patterns: Array<{
    regex: RegExp;
    severity: DiagnosticSeverity;
    message: string;
    skipComments: boolean;
    skipStrings: boolean;
}> = [
    // Invalid number exponent (e.g., 0e$, 1e-, 3.14e)
    {
        regex: /\b\d+(\.\d+)?[eE][+-]?(?!\d)/g,
        severity: DiagnosticSeverity.Error,
        message: "Invalid numeric exponent: missing or incomplete exponent digits",
        skipComments: true,
        skipStrings: true,
    },
    // Multiple decimal points (e.g., 1.2.3)
    {
        regex: /\b\d+\.\d+\.\d*\b/g,
        severity: DiagnosticSeverity.Error,
        message: "Multiple decimal points in number",
        skipComments: true,
        skipStrings: true,
    },
    // Invalid hex literal (e.g., 0xGG)
    {
        regex: /\b0[xX][^0-9a-fA-F_]+\b/g,
        severity: DiagnosticSeverity.Error,
        message: "Invalid hexadecimal literal",
        skipComments: true,
        skipStrings: true,
    },
    // Invalid binary literal (e.g., 0b2)
    {
        regex: /\b0[bB][^01_]+\b/g,
        severity: DiagnosticSeverity.Error,
        message: "Invalid binary literal",
        skipComments: true,
        skipStrings: true,
    },
    // Invalid octal literal (e.g., 0o88)
    {
        regex: /\b0[oO][^0-7_]+\b/g,
        severity: DiagnosticSeverity.Error,
        message: "Invalid octal literal",
        skipComments: true,
        skipStrings: true,
    },
    // Number immediately followed by identifier without operator (e.g., 123abc)
    {
        regex: /\b\d+\.?\d*\s+(?!is\b|do\b|to\b|n\b|times\b|then\b|else\b|optarg\b|env\b|param\b|and\b|or\b)[a-zA-Z_]\w*\b/g,
        severity: DiagnosticSeverity.Warning,
        message: "Number followed by identifier without operator",
        skipComments: true,
        skipStrings: true,
    },
    // Module keyword typo (e.g., "autp" instead of "auto")
    {
        regex: /\bmodule\b\s+"[^"]*"\s+\b(?!auto\b|as\b)[a-zA-Z_]\w*\b/g,
        severity: DiagnosticSeverity.Warning,
        message: "Possible typo in module clause (expected 'auto' or 'as')",
        skipComments: true,
        skipStrings: true,
    },
    // Doc keyword typo (e.g., "delm" instead of "delim")
    {
        regex: /\bdoc\b\s+"[^"]*"\s+\b(?!delim\b|gen\b|var\b)[a-zA-Z_]\w*\b/g,
        severity: DiagnosticSeverity.Warning,
        message: "Possible typo in doc clause (expected 'delim', 'gen', or 'var')",
        skipComments: true,
        skipStrings: true,
    },
    // Bare $ identifier
    {
        regex: /\b\$\b/g,
        severity: DiagnosticSeverity.Warning,
        message: "Bare '$' identifier — possible typo or incomplete variable name",
        skipComments: true,
        skipStrings: true,
    },
];

export function createDiagnosticCollection(): DiagnosticCollection {
    diagnosticCollection = languages.createDiagnosticCollection('za-lint');
    return diagnosticCollection;
}

export function validateDocument(document: TextDocument): Diagnostic[] {
    const diagnostics: Diagnostic[] = [];
    const text = document.getText();
    const lines = text.split('\n');

    for (let lineIndex = 0; lineIndex < lines.length; lineIndex++) {
        const line = lines[lineIndex];
        const lineStart = document.positionAt(
            text.split('\n').slice(0, lineIndex).reduce((sum, l) => sum + l.length + 1, 0)
        );

        for (const pattern of patterns) {
            if (pattern.skipComments && line.trimStart().startsWith('#')) {
                continue;
            }

            let searchLine = line;
            if (pattern.skipStrings) {
                searchLine = stripStrings(line);
            }

            pattern.regex.lastIndex = 0;
            let match: RegExpExecArray | null;
            while ((match = pattern.regex.exec(searchLine)) !== null) {
                const startCol = match.index;
                const endCol = startCol + match[0].length;
                const range = new Range(
                    lineIndex,
                    startCol,
                    lineIndex,
                    endCol
                );
                diagnostics.push(new Diagnostic(range, pattern.message, pattern.severity));
            }
        }
    }

    return diagnostics;
}

function stripStrings(line: string): string {
    // Replace content inside "..." and `...` with spaces to preserve positions
    let result = line;

    // Double-quoted strings
    const dqRegex = /"(?:[^"\\]|\\.)*"/g;
    result = result.replace(dqRegex, (m) => ' '.repeat(m.length));

    // Backtick strings
    const btRegex = /`(?:[^`\\]|\\.)*`/g;
    result = result.replace(btRegex, (m) => ' '.repeat(m.length));

    // Single-quoted strings
    const sqRegex = /'(?:[^'\\]|\\.)*'/g;
    result = result.replace(sqRegex, (m) => ' '.repeat(m.length));

    return result;
}

export function updateDiagnostics(document: TextDocument) {
    if (!diagnosticCollection) {
        return;
    }
    const diagnostics = validateDocument(document);
    diagnosticCollection.set(document.uri, diagnostics);
}

export function clearDiagnostics(document: TextDocument) {
    if (!diagnosticCollection) {
        return;
    }
    diagnosticCollection.delete(document.uri);
}

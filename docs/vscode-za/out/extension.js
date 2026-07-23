"use strict";
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const path = require("path");
const vscode_1 = require("vscode");
const node_1 = require("vscode-languageclient/node");
const diagnostics_1 = require("./diagnostics");
let client;
let diagnosticTimeout;
function findZaBinary(outputChannel) {
    const fs = require('fs');
    const { execSync } = require('child_process');
    try {
        const zaPath = execSync('which za', { encoding: 'utf8' }).trim();
        if (fs.existsSync(zaPath)) {
            return zaPath;
        }
    }
    catch {
        // not in PATH
    }
    const candidates = [
        '/home/daniel/go/src/za/za',
        '/home/daniel/go/src/za/za_fixed',
        '/home/daniel/go/src/za/za_final',
        '/home/daniel/go/src/za/za_final2',
        '/tmp/za',
        '/tmp/za_final',
        '/tmp/za_final2',
        '/usr/local/bin/za',
        '/usr/bin/za',
    ];
    for (const candidate of candidates) {
        if (fs.existsSync(candidate)) {
            return candidate;
        }
    }
    return null;
}
async function activate(context) {
    const outputChannel = vscode_1.window.createOutputChannel('ZA Language Extension');
    outputChannel.appendLine('[ZA] Extension activating...');
    // Initialize client-side diagnostics
    (0, diagnostics_1.createDiagnosticCollection)();
    // Run diagnostics on all open Za documents
    const zaDocs = vscode_1.workspace.textDocuments.filter(doc => doc.languageId === 'za');
    for (const doc of zaDocs) {
        (0, diagnostics_1.updateDiagnostics)(doc);
    }
    // Update diagnostics on document open and change
    context.subscriptions.push(vscode_1.workspace.onDidOpenTextDocument((doc) => {
        if (doc.languageId === 'za') {
            (0, diagnostics_1.updateDiagnostics)(doc);
        }
    }));
    context.subscriptions.push(vscode_1.workspace.onDidChangeTextDocument((event) => {
        if (event.document.languageId !== 'za') {
            return;
        }
        if (diagnosticTimeout) {
            clearTimeout(diagnosticTimeout);
        }
        diagnosticTimeout = setTimeout(() => {
            (0, diagnostics_1.updateDiagnostics)(event.document);
        }, 200);
    }));
    context.subscriptions.push(vscode_1.workspace.onDidCloseTextDocument((doc) => {
        if (doc.languageId === 'za') {
            (0, diagnostics_1.clearDiagnostics)(doc);
        }
    }));
    const serverPath = path.join(context.extensionPath, 'bin', 'za-lsp');
    outputChannel.appendLine(`[ZA] Looking for server at: ${serverPath}`);
    const fs = require('fs');
    let actualServerPath = serverPath;
    if (!fs.existsSync(serverPath)) {
        outputChannel.appendLine('[ZA] Server not found in extension bin, trying dev path...');
        const devPath = path.join(context.extensionPath, '..', '..', 'lsp', 'za-lsp');
        if (fs.existsSync(devPath)) {
            actualServerPath = devPath;
            outputChannel.appendLine(`[ZA] Found dev server at: ${actualServerPath}`);
        }
        else {
            outputChannel.appendLine('[ZA] Server not found in dev path, trying PATH...');
            const { execSync } = require('child_process');
            try {
                actualServerPath = execSync('which za-lsp', { encoding: 'utf8' }).trim();
                outputChannel.appendLine(`[ZA] Found server in PATH at: ${actualServerPath}`);
            }
            catch {
                outputChannel.appendLine('[ZA] ERROR: za-lsp binary not found!');
                outputChannel.show();
                return;
            }
        }
    }
    else {
        outputChannel.appendLine(`[ZA] Server found at: ${actualServerPath}`);
    }
    const zaPath = findZaBinary(outputChannel);
    if (zaPath) {
        outputChannel.appendLine(`[ZA] Found za binary at: ${zaPath}`);
    }
    else {
        outputChannel.appendLine('[ZA] WARNING: za binary not found, server may fail to load stdlib metadata');
    }
    const serverArgs = zaPath ? [zaPath] : [];
    const lspOutputChannel = vscode_1.window.createOutputChannel('ZA Language Server');
    const logFile = path.join(context.extensionPath, 'server.log');
    const serverEnv = { ...process.env, ZA_LSP_LOG: logFile };
    const serverOptions = {
        run: {
            command: actualServerPath,
            args: serverArgs,
            transport: node_1.TransportKind.stdio,
            options: {
                env: serverEnv,
            }
        },
        debug: {
            command: actualServerPath,
            args: serverArgs,
            transport: node_1.TransportKind.stdio,
            options: {
                env: serverEnv,
            }
        }
    };
    const clientOptions = {
        documentSelector: [{ scheme: 'file', language: 'za' }],
        synchronize: {
            fileEvents: vscode_1.workspace.createFileSystemWatcher('**/*.za')
        },
        outputChannel: lspOutputChannel,
        traceOutputChannel: lspOutputChannel,
    };
    client = new node_1.LanguageClient('zaLanguageServer', 'ZA Language Server', serverOptions, clientOptions);
    outputChannel.appendLine('[ZA] Starting LanguageClient...');
    try {
        await client.start();
        outputChannel.appendLine('[ZA] LanguageClient started successfully');
    }
    catch (err) {
        outputChannel.appendLine(`[ZA] ERROR starting LanguageClient: ${err}`);
        if (err && err.message) {
            outputChannel.appendLine(`[ZA] Error message: ${err.message}`);
        }
        if (err && err.stack) {
            outputChannel.appendLine(`[ZA] Stack: ${err.stack}`);
        }
        outputChannel.show();
    }
}
function deactivate() {
    if (!client) {
        return undefined;
    }
    return client.stop();
}
//# sourceMappingURL=extension.js.map
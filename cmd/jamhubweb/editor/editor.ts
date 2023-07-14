import { keymap } from "@codemirror/view";
import { indentWithTab } from "@codemirror/commands";
import { StreamLanguage } from "@codemirror/language";
import { go } from "@codemirror/legacy-modes/mode/go"
import { tags } from "@lezer/highlight"
import { HighlightStyle } from "@codemirror/language"
import { syntaxHighlighting } from "@codemirror/language"
import { vim } from "@replit/codemirror-vim";
import { basicSetup } from "codemirror"
import { EditorState } from "@codemirror/state"
import { EditorView } from "@codemirror/view"

let splitPath = window.location.pathname.split("/");
let owner = splitPath[2];
let projectName = splitPath[3];

const myHighlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--tan)" },
  { tag: tags.comment, color: "#00ff80" },
  { tag: tags.string, color: "var(--bright-blue)" }
])

let myTheme = EditorView.theme({
  "&": {
    color: "white",
    backgroundColor: "var(--dark-blue)",
    font: "'Fira Code', Monaco, Consolas, Ubuntu Mono, monospace",
  },
  ".cm-editor.cm-focused": {
    outline: "1px solid var(--bright-pink)",
  },
  ".cm-content": {
    caretColor: "white",
  },
  ".cm-activeLineGutter": {
    backgroundColor: "rgba(255, 0, 127, 0.25)",
  },
  ".cm-activeLine": {
    backgroundColor: "rgba(255, 0, 127, 0.25)",
  },
  "&.cm-focused .cm-cursor": {
    borderLeftColor: "var(--bright-pink)",
  },
  "&.cm-focused .cm-selectionBackground, ::selection": {
    backgroundColor: "#ff007f",
  },
  ".cm-selectionMatch": {
    backgroundColor: "#ff007f",
  },
  ".cm-gutters": {
    backgroundColor: "var(--dark-blue)",
    color: "#ddd",
    border: "none",
  },
}, { dark: true });

async function setup() {
  let fileResp;
  if (window.location.pathname.includes("committedfile")) {
    const currentPath = splitPath.slice(4).join("/");
    const currentCommitResp = await fetch(`/api/projects/${owner}/${projectName}`);
    const currentCommitJson = await currentCommitResp.json();
    const currentCommitId = currentCommitJson.commit_id ?? 0;
    fileResp = await fetch(
      `/api/committedfile/${owner}/${projectName}/${currentCommitId}?path=${currentPath}`,
    );
  } else {
    const workspaceName = splitPath[4];
    const workspaceInfoResp = await fetch(`/api/workspaces/${owner}/${projectName}/${workspaceName}`);
    const workspaceInfoJson = await workspaceInfoResp.json();
    const currentChangeId = workspaceInfoJson.change_id ?? 0;
    const workspaceId = workspaceInfoJson.workspace_id ?? 0;
    const currentPath = splitPath.slice(5).join("/");
    fileResp = await fetch(
      `/api/workspacefile/${owner}/${projectName}/${workspaceId}/${currentChangeId}?path=${currentPath}`,
    );
  }
  const doc = await fileResp.text();
  const extensions = [
    vim(),
    myTheme,
    basicSetup,
    keymap.of([indentWithTab]),
    EditorState.readOnly.of(true),
  ]
  // if (currentPath.endsWith('.go')) {
  //   extensions.push(StreamLanguage.define(go))
  //   extensions.push(syntaxHighlighting(myHighlightStyle))
  // }
  let state = EditorState.create({
    doc,
    extensions: extensions,
  })
  let editors = document.querySelector("#js-file-location")
  editors.innerHTML = ""
  let wrap = editors.appendChild(document.createElement("div"))
  wrap.className = "editor"
  new EditorView({ state, parent: wrap })
}

setup()
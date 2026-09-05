import { defineConfig } from "vitepress";

export default defineConfig({
  title: "SP Local Go Bridge",
  description:
    "Control Super Productivity from MCP hosts, CLI, and automation tools — single binary, zero dependencies.",
  base: "/super-productivity-local-gobridge/",

  head: [
    ["meta", { name: "theme-color", content: "#4385f4" }],
    ["meta", { name: "og:type", content: "website" }],
    [
      "meta",
      {
        name: "og:title",
        content: "SP Local Go Bridge — Single-binary task automation",
      },
    ],
    [
      "meta",
      {
        name: "og:description",
        content:
          "Control Super Productivity from MCP hosts, CLI, and automation tools. Zero dependencies.",
      },
    ],
  ],

  themeConfig: {
    siteTitle: "SP Go Bridge",

    nav: [
      { text: "Guide", link: "/getting-started" },
      { text: "Operations", link: "/operations" },
      { text: "Hosts", link: "/hosts/" },
      {
        text: "v0.1.1",
        items: [
          { text: "Changelog", link: "https://github.com/CameronBrooks11/super-productivity-local-gobridge/releases" },
          { text: "Python Bridge (archived)", link: "https://cameronbrooks11.github.io/super-productivity-local-bridge/" },
        ],
      },
    ],

    sidebar: [
      {
        text: "Getting Started",
        items: [
          { text: "Quick Start", link: "/getting-started" },
          { text: "Install", link: "/install" },
          { text: "Migration from Python", link: "/migration" },
        ],
      },
      {
        text: "Host Setup",
        items: [
          { text: "Overview", link: "/hosts/" },
          { text: "Claude Code", link: "/hosts/claude-code" },
          { text: "VS Code Copilot", link: "/hosts/vscode-copilot" },
          { text: "Claude Desktop", link: "/hosts/claude-desktop" },
          { text: "Codex CLI", link: "/hosts/codex" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "Operations", link: "/operations" },
          { text: "Architecture", link: "/architecture" },
          { text: "Security", link: "/security" },
          { text: "Troubleshooting", link: "/troubleshooting" },
        ],
      },
      {
        text: "Project",
        items: [
          { text: "Validation Status", link: "/validation-status" },
        ],
      },
    ],

    socialLinks: [
      {
        icon: "github",
        link: "https://github.com/CameronBrooks11/super-productivity-local-gobridge",
      },
    ],

    search: {
      provider: "local",
    },

    footer: {
      message: "Released under the MIT License.",
      copyright: "A local-first automation bridge for Super Productivity.",
    },

    editLink: {
      pattern:
        "https://github.com/CameronBrooks11/super-productivity-local-gobridge/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },
  },
});

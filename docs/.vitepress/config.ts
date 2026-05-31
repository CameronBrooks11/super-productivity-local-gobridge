import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Super Productivity Local Go Bridge",
  description:
    "Control Super Productivity from MCP hosts, CLI, and automation tools — single binary, zero dependencies.",
  base: "/super-productivity-local-gobridge/",
  themeConfig: {
    nav: [
      { text: "Home", link: "/" },
      { text: "Getting Started", link: "/getting-started" },
      { text: "Operations", link: "/operations" },
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "Getting Started", link: "/getting-started" },
          { text: "Install", link: "/install" },
          { text: "Operations", link: "/operations" },
          { text: "Architecture", link: "/architecture" },
          { text: "Migration from Python", link: "/migration" },
        ],
      },
      {
        text: "Host Setup",
        items: [
          { text: "Overview", link: "/hosts/" },
          { text: "VS Code Copilot", link: "/hosts/vscode-copilot" },
          { text: "Claude Desktop", link: "/hosts/claude-desktop" },
          { text: "Codex CLI", link: "/hosts/codex" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "Validation Status", link: "/validation-status" },
          { text: "Security", link: "/security" },
          { text: "Troubleshooting", link: "/troubleshooting" },
        ],
      },
    ],
    socialLinks: [
      {
        icon: "github",
        link: "https://github.com/CameronBrooks11/super-productivity-local-gobridge",
      },
    ],
  },
});

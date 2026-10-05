import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { MarkdownContent } from "./markdown-content";

describe("MarkdownContent", () => {
  it("renders plain text as paragraph", () => {
    render(<MarkdownContent content="Hello world" />);
    expect(screen.getByText("Hello world")).toBeInTheDocument();
  });

  it("renders markdown headings one level down", () => {
    render(<MarkdownContent content="## My Heading" />);
    const heading = screen.getByRole("heading", { level: 3 });
    expect(heading).toHaveTextContent("My Heading");
  });

  // User content sits under the page's own title, the page's one <h1>: every heading the
  // author wrote moves one level down, so a body that starts with "#" never adds a second h1.
  it.each(["default", "compact"] as const)("never renders an h1 from user content (%s)", (variant) => {
    const content = ["# One", "## Two", "### Three", "#### Four", "##### Five", "###### Six"].join("\n\n");
    render(<MarkdownContent content={content} variant={variant} />);
    expect(screen.queryAllByRole("heading", { level: 1 })).toHaveLength(0);
    expect(screen.getByRole("heading", { level: 2 })).toHaveTextContent("One");
    expect(screen.getByRole("heading", { level: 3 })).toHaveTextContent("Two");
    expect(screen.getByRole("heading", { level: 4 })).toHaveTextContent("Three");
    expect(screen.getByRole("heading", { level: 5 })).toHaveTextContent("Four");
    expect(screen.getAllByRole("heading", { level: 6 }).map((h) => h.textContent)).toEqual(["Five", "Six"]);
  });

  it("passes no react-markdown internals to the heading elements", () => {
    const { container } = render(<MarkdownContent content="# Title" />);
    expect(container.querySelector("h2")?.getAttribute("node")).toBeNull();
  });

  it("renders bold text", () => {
    render(<MarkdownContent content="This is **bold** text" />);
    const bold = screen.getByText("bold");
    expect(bold.tagName).toBe("STRONG");
  });

  it("renders inline code", () => {
    render(<MarkdownContent content="Use `console.log`" />);
    const code = screen.getByText("console.log");
    expect(code.tagName).toBe("CODE");
  });

  it("renders code blocks", () => {
    const content = "```\nconst x = 1;\n```";
    render(<MarkdownContent content={content} />);
    expect(screen.getByText("const x = 1;")).toBeInTheDocument();
  });

  it("renders unordered lists", () => {
    render(<MarkdownContent content={"- item one\n- item two"} />);
    const items = screen.getAllByRole("listitem");
    expect(items.length).toBeGreaterThanOrEqual(1);
  });

  it("renders ordered lists", () => {
    render(<MarkdownContent content="1. first\n2. second" />);
    const list = screen.getByRole("list");
    expect(list.tagName).toBe("OL");
  });

  it("renders links", () => {
    render(<MarkdownContent content="[click here](https://example.com)" />);
    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "https://example.com");
    expect(link).toHaveTextContent("click here");
  });

  it("does NOT render raw HTML (security)", () => {
    render(<MarkdownContent content='<script>alert("xss")</script>' />);
    expect(screen.queryByText('alert("xss")')).not.toBeInTheDocument();
    // Script tag should not be in the DOM
    const container = document.querySelector("script");
    expect(container).toBeNull();
  });

  it("handles empty string", () => {
    const { container } = render(<MarkdownContent content="" />);
    expect(container.firstChild).toBeInTheDocument();
  });

  it("applies default variant prose classes", () => {
    const { container } = render(<MarkdownContent content="test" />);
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.className).toContain("prose");
    expect(wrapper.className).toContain("prose-invert");
  });

  it("applies compact variant classes", () => {
    const { container } = render(
      <MarkdownContent content="test" variant="compact" />
    );
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.className).toContain("prose-xs");
  });

  it("accepts custom className", () => {
    const { container } = render(
      <MarkdownContent content="test" className="mb-8" />
    );
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper.className).toContain("mb-8");
  });

  it("renders blockquotes", () => {
    render(<MarkdownContent content="> This is a quote" />);
    const blockquote = screen.getByText("This is a quote");
    expect(blockquote.closest("blockquote")).toBeInTheDocument();
  });
});

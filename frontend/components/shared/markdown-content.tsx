import type { ComponentPropsWithoutRef } from "react";
import Markdown, { type Components, type ExtraProps } from "react-markdown";
import { cn } from "@/lib/utils";

interface MarkdownContentProps {
  content: string;
  className?: string;
  variant?: "default" | "compact";
}

const defaultStyles = [
  "prose prose-invert prose-sm sm:prose-base max-w-none",
  // A heading keeps the size of its Markdown level: "#" renders as h2 (see HEADINGS below).
  "[&_h2]:text-2xl [&_h2]:font-light [&_h2]:tracking-tight",
  "[&_h3]:text-xl [&_h3]:font-light",
  "[&_h4]:text-lg [&_h4]:font-light",
  "[&_p]:text-muted-foreground [&_p]:leading-relaxed [&_p]:whitespace-pre-line",
  "[&_a]:text-foreground [&_a]:underline [&_a]:underline-offset-4",
  "[&_code]:font-mono [&_code]:text-sm [&_code]:bg-secondary [&_code]:px-1.5 [&_code]:py-0.5",
  "[&_pre]:bg-secondary [&_pre]:border [&_pre]:border-border [&_pre]:overflow-x-auto",
  "[&_pre_code]:bg-transparent [&_pre_code]:px-0 [&_pre_code]:py-0",
  "[&_blockquote]:border-l-2 [&_blockquote]:border-foreground [&_blockquote]:pl-4 [&_blockquote]:italic",
  "[&_ul]:list-disc [&_ol]:list-decimal [&_li]:text-muted-foreground",
].join(" ");

const compactStyles = [
  "prose prose-invert prose-xs max-w-none",
  "[&_p]:text-muted-foreground [&_p]:leading-relaxed [&_p]:whitespace-pre-line",
  "[&_a]:text-foreground [&_a]:underline [&_a]:underline-offset-4",
  "[&_code]:font-mono [&_code]:text-xs [&_code]:bg-secondary [&_code]:px-1 [&_code]:py-0.5",
  "[&_pre]:bg-secondary [&_pre]:border [&_pre]:border-border [&_pre]:overflow-x-auto",
  "[&_pre_code]:bg-transparent [&_pre_code]:px-0 [&_pre_code]:py-0",
  "[&_blockquote]:border-l-2 [&_blockquote]:border-foreground [&_blockquote]:pl-4 [&_blockquote]:italic",
  "[&_ul]:list-disc [&_ol]:list-decimal [&_li]:text-muted-foreground",
].join(" ");

type HeadingTag = "h2" | "h3" | "h4" | "h5" | "h6";

// shiftedHeading renders a Markdown heading as the given, lower element, and leaves out the
// syntax-tree node react-markdown passes along.
function shiftedHeading(Tag: HeadingTag) {
  function ShiftedHeading(props: ComponentPropsWithoutRef<HeadingTag> & ExtraProps) {
    const attributes = { ...props };
    delete attributes.node;
    return <Tag {...attributes} />;
  }
  ShiftedHeading.displayName = `Shifted(${Tag})`;
  return ShiftedHeading;
}

// User content sits under its page's title, which is the page's one <h1>. Every heading the
// author wrote moves one level down ("#" becomes h2, "######" stays h6), so a body that starts
// with "#" can never give the page a second h1.
const HEADINGS: Components = {
  h1: shiftedHeading("h2"),
  h2: shiftedHeading("h3"),
  h3: shiftedHeading("h4"),
  h4: shiftedHeading("h5"),
  h5: shiftedHeading("h6"),
  h6: shiftedHeading("h6"),
};

export function MarkdownContent({
  content,
  className,
  variant = "default",
}: MarkdownContentProps) {
  const styles = variant === "compact" ? compactStyles : defaultStyles;

  return (
    <div className={cn(styles, className)}>
      <Markdown components={HEADINGS}>{content ?? ""}</Markdown>
    </div>
  );
}

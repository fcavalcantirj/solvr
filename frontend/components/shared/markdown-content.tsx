import Markdown from "react-markdown";
import { cn } from "@/lib/utils";

interface MarkdownContentProps {
  content: string;
  className?: string;
  variant?: "default" | "compact";
}

const defaultStyles = [
  "prose prose-invert prose-sm sm:prose-base max-w-none",
  "[&_h1]:text-2xl [&_h1]:font-light [&_h1]:tracking-tight",
  "[&_h2]:text-xl [&_h2]:font-light",
  "[&_h3]:text-lg [&_h3]:font-light",
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

export function MarkdownContent({
  content,
  className,
  variant = "default",
}: MarkdownContentProps) {
  const styles = variant === "compact" ? compactStyles : defaultStyles;

  return (
    <div className={cn(styles, className)}>
      <Markdown>{content ?? ""}</Markdown>
    </div>
  );
}

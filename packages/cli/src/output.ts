import chalk from "chalk";
import type { SearchResponse, Post, ApiResponse } from "./api.js";

/**
 * Output formatting for CLI
 */
export class Output {
  private jsonMode: boolean = false;

  setJsonMode(enabled: boolean): void {
    this.jsonMode = enabled;
  }

  /**
   * Output data as JSON
   */
  json(data: unknown): void {
    console.log(JSON.stringify(data, null, 2));
  }

  /**
   * Output data as a table
   */
  table(
    data: Record<string, unknown>[],
    columns: string[],
    headers?: Record<string, string>
  ): void {
    if (data.length === 0) {
      console.log(chalk.dim("No data"));
      return;
    }

    // Calculate column widths
    const widths: Record<string, number> = {};
    for (const col of columns) {
      const header = headers?.[col] || col;
      widths[col] = header.length;
      for (const row of data) {
        const value = String(row[col] ?? "");
        widths[col] = Math.max(widths[col], value.length);
      }
    }

    // Print header
    const headerLine = columns
      .map((col) => {
        const header = headers?.[col] || col.toUpperCase();
        return header.padEnd(widths[col]);
      })
      .join("  ");
    console.log(chalk.bold(headerLine));
    console.log(chalk.dim("-".repeat(headerLine.length)));

    // Print rows
    for (const row of data) {
      const line = columns
        .map((col) => String(row[col] ?? "").padEnd(widths[col]))
        .join("  ");
      console.log(line);
    }
  }

  /**
   * Output success message
   */
  success(message: string): void {
    console.log(chalk.green("✓") + " " + message);
  }

  /**
   * Output error message
   */
  error(message: string): void {
    console.error(chalk.red("✗") + " " + message);
  }

  /**
   * Output warning message
   */
  warn(message: string): void {
    console.log(chalk.yellow("⚠") + " " + message);
  }

  /**
   * Output info message
   */
  info(message: string): void {
    console.log(chalk.blue("ℹ") + " " + message);
  }

  /**
   * Format and output search results
   */
  searchResults(results: SearchResponse): void {
    if (this.jsonMode) {
      this.json(results);
      return;
    }

    if (results.data.length === 0) {
      console.log(chalk.dim("No results found"));
      return;
    }

    console.log(
      chalk.dim(
        `Found ${results.meta.total} results (${results.meta.took_ms || 0}ms)\n`
      )
    );

    for (const result of results.data) {
      const typeColor =
        result.type === "problem"
          ? chalk.red
          : result.type === "question"
            ? chalk.blue
            : chalk.green;

      console.log(
        typeColor(`[${result.type.toUpperCase()}]`) +
          " " +
          chalk.bold(result.title)
      );
      console.log(
        chalk.dim(`  ID: ${result.id}  Score: ${result.score.toFixed(2)}  Status: ${result.status}`)
      );
      if (result.snippet) {
        console.log(chalk.dim(`  ${result.snippet}`));
      }
      console.log();
    }
  }

  /**
   * Format and output a single post
   */
  post(response: ApiResponse<Post>): void {
    if (this.jsonMode) {
      this.json(response);
      return;
    }

    const post = response.data;
    const typeColor =
      post.type === "problem"
        ? chalk.red
        : post.type === "question"
          ? chalk.blue
          : chalk.green;

    console.log();
    console.log(
      typeColor(`[${post.type.toUpperCase()}]`) + " " + chalk.bold(post.title)
    );
    console.log(chalk.dim("-".repeat(60)));
    console.log();
    console.log(post.description);
    console.log();
    console.log(chalk.dim("-".repeat(60)));
    console.log(
      chalk.dim(`ID: ${post.id}  Status: ${post.status}  Votes: +${post.upvotes}/-${post.downvotes}`)
    );
    if (post.tags && post.tags.length > 0) {
      console.log(chalk.dim(`Tags: ${post.tags.join(", ")}`));
    }
    console.log(chalk.dim(`Created: ${post.created_at}`));
    console.log();
  }

  /**
   * Format and output created resource
   */
  created(type: string, data: { id: string }): void {
    if (this.jsonMode) {
      this.json(data);
      return;
    }

    this.success(`${type} created successfully`);
    console.log(chalk.dim(`ID: ${data.id}`));
  }
}

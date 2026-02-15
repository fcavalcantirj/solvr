import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { FeedFilters, FilterState } from "./feed-filters";

const defaultFilters: FilterState = {
  type: "all",
  status: "All",
  sort: "Newest",
  timeframe: "All Time",
  searchQuery: "",
  viewMode: "list",
};

describe("FeedFilters", () => {
  let onFiltersChange: (filters: Partial<FilterState>) => void;
  let onToggleFilters: () => void;

  beforeEach(() => {
    onFiltersChange = vi.fn();
    onToggleFilters = vi.fn();
  });

  it("toggles mobile search on click of search button", () => {
    const { container } = render(
      <FeedFilters
        filters={defaultFilters}
        onFiltersChange={onFiltersChange}
        showFilters={false}
        onToggleFilters={onToggleFilters}
      />
    );

    // Find the mobile search toggle button (sm:hidden with Search icon)
    const mobileSearchBtn = container.querySelector("button.sm\\:hidden");
    expect(mobileSearchBtn).toBeTruthy();

    // Initially mobile search input should not be visible
    const mobileSearchInputBefore = container.querySelector(
      ".sm\\:hidden input[placeholder='Search feed...']"
    );
    expect(mobileSearchInputBefore).toBeNull();

    // Click the search toggle
    fireEvent.click(mobileSearchBtn!);

    // After click, mobile search input should be visible
    const mobileSearchInputAfter = container.querySelector(
      ".sm\\:hidden input[placeholder='Search feed...']"
    );
    expect(mobileSearchInputAfter).toBeTruthy();
  });

  it("hides mobile search when toggle is clicked again", () => {
    const { container } = render(
      <FeedFilters
        filters={defaultFilters}
        onFiltersChange={onFiltersChange}
        showFilters={false}
        onToggleFilters={onToggleFilters}
      />
    );

    const mobileSearchBtn = container.querySelector("button.sm\\:hidden");

    // Open mobile search
    fireEvent.click(mobileSearchBtn!);
    let mobileSearchInput = container.querySelector(
      ".sm\\:hidden input[placeholder='Search feed...']"
    );
    expect(mobileSearchInput).toBeTruthy();

    // Close mobile search
    fireEvent.click(mobileSearchBtn!);
    mobileSearchInput = container.querySelector(
      ".sm\\:hidden input[placeholder='Search feed...']"
    );
    expect(mobileSearchInput).toBeNull();
  });

  it("does not render redundant mobile type dropdown with ChevronDown", () => {
    const { container } = render(
      <FeedFilters
        filters={defaultFilters}
        onFiltersChange={onFiltersChange}
        showFilters={false}
        onToggleFilters={onToggleFilters}
      />
    );

    // The redundant mobile dropdown (md:hidden div with ChevronDown button) should NOT exist
    // It was a non-functional dropdown that duplicated the mobile horizontal scroll tabs
    const chevronDownElements = container.querySelectorAll(
      ".md\\:hidden button"
    );
    // The only md:hidden buttons should be the horizontal scroll type tabs, not a dropdown
    chevronDownElements.forEach((btn) => {
      // None of these buttons should contain a ChevronDown SVG (the dropdown indicator)
      const svgs = btn.querySelectorAll("svg");
      svgs.forEach((svg) => {
        // ChevronDown has a specific path — check there's no "chevron-down" class or similar indicator
        expect(btn.textContent).not.toContain("ChevronDown");
      });
    });
  });

  it("mobile horizontal scroll tabs still work for type selection", () => {
    const { container } = render(
      <FeedFilters
        filters={defaultFilters}
        onFiltersChange={onFiltersChange}
        showFilters={false}
        onToggleFilters={onToggleFilters}
      />
    );

    // Find the mobile horizontal scroll section
    const mobileTabsContainer = container.querySelector(
      ".md\\:hidden.overflow-x-auto"
    );
    expect(mobileTabsContainer).toBeTruthy();

    // Find the PROBLEMS tab button
    const problemsTab = Array.from(
      mobileTabsContainer!.querySelectorAll("button")
    ).find((btn) => btn.textContent === "PROBLEMS");
    expect(problemsTab).toBeTruthy();

    fireEvent.click(problemsTab!);
    expect(onFiltersChange).toHaveBeenCalledWith({ type: "problem" });
  });

  it("mobile search input triggers onFiltersChange with searchQuery", () => {
    const { container } = render(
      <FeedFilters
        filters={defaultFilters}
        onFiltersChange={onFiltersChange}
        showFilters={false}
        onToggleFilters={onToggleFilters}
      />
    );

    // Open mobile search
    const mobileSearchBtn = container.querySelector("button.sm\\:hidden");
    fireEvent.click(mobileSearchBtn!);

    // Type in the mobile search input
    const mobileSearchInput = container.querySelector(
      ".sm\\:hidden input[placeholder='Search feed...']"
    );
    expect(mobileSearchInput).toBeTruthy();

    fireEvent.change(mobileSearchInput!, { target: { value: "test query" } });
    expect(onFiltersChange).toHaveBeenCalledWith({
      searchQuery: "test query",
    });
  });
});

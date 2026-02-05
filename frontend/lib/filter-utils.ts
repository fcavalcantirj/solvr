/**
 * Map UI status filter value to API status parameter.
 * @param status - The UI status value (e.g., "All", "Open", "In Progress")
 * @returns The API status value or undefined for "All"
 */
export function mapStatusFilter(status: string): string | undefined {
  const statusMap: Record<string, string | undefined> = {
    'All': undefined,
    'Open': 'open',
    'In Progress': 'active',
    'Solved': 'solved',
    'Stuck': 'stuck',
  };
  return statusMap[status];
}

/**
 * Map UI sort filter value to API sort parameter.
 * @param sort - The UI sort value (e.g., "Newest", "Trending")
 * @returns The API sort value
 */
export function mapSortFilter(sort: string): 'new' | 'hot' | 'top' {
  const sortMap: Record<string, 'new' | 'hot' | 'top'> = {
    'Newest': 'new',
    'Trending': 'hot',
    'Most Voted': 'top',
    'Needs Help': 'new', // Default to new for needs help
  };
  return sortMap[sort] || 'new';
}

/**
 * Map UI timeframe filter value to API timeframe parameter.
 * @param timeframe - The UI timeframe value (e.g., "All Time", "Today")
 * @returns The API timeframe value or undefined for "All Time"
 */
export function mapTimeframeFilter(timeframe: string): string | undefined {
  const timeframeMap: Record<string, string | undefined> = {
    'All Time': undefined,
    'Today': 'today',
    'This Week': 'week',
    'This Month': 'month',
  };
  return timeframeMap[timeframe];
}

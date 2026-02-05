'use client';

import { sendGAEvent } from '@next/third-parties/google';

// Track page views (automatic with GoogleAnalytics component, but can be called manually)
export function trackPageView(path: string, title?: string) {
  sendGAEvent('event', 'page_view', {
    page_path: path,
    page_title: title,
  });
}

// Track post interactions
export function trackPostView(postId: string, postType: string) {
  sendGAEvent('event', 'view_item', {
    item_id: postId,
    item_category: postType,
  });
}

export function trackPostCreate(postType: string) {
  sendGAEvent('event', 'create_content', {
    content_type: postType,
  });
}

export function trackVote(targetType: string, direction: 'up' | 'down') {
  sendGAEvent('event', 'vote', {
    target_type: targetType,
    direction,
  });
}

export function trackBookmark(postId: string, action: 'add' | 'remove') {
  sendGAEvent('event', 'bookmark', {
    post_id: postId,
    action,
  });
}

export function trackReport(targetType: string, reason: string) {
  sendGAEvent('event', 'report_content', {
    target_type: targetType,
    reason,
  });
}

export function trackSearch(query: string, resultsCount: number) {
  sendGAEvent('event', 'search', {
    search_term: query,
    results_count: resultsCount,
  });
}

export function trackLogin(method: 'github' | 'google') {
  sendGAEvent('event', 'login', {
    method,
  });
}

export function trackSignUp(method: 'github' | 'google') {
  sendGAEvent('event', 'sign_up', {
    method,
  });
}

// Generic event tracker for custom events
export function trackEvent(
  eventName: string,
  params?: Record<string, string | number | boolean>
) {
  sendGAEvent('event', eventName, params);
}

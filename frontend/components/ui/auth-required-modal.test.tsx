import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { AuthRequiredModal } from './auth-required-modal';

// Mock localStorage
const localStorageMock = (() => {
  let store: Record<string, string> = {};
  return {
    getItem: vi.fn((key: string) => store[key] || null),
    setItem: vi.fn((key: string, value: string) => { store[key] = value; }),
    removeItem: vi.fn((key: string) => { delete store[key]; }),
    clear: vi.fn(() => { store = {}; }),
  };
})();

Object.defineProperty(window, 'localStorage', { value: localStorageMock });

describe('AuthRequiredModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorageMock.clear();
  });

  it('should render with default message when open', () => {
    render(
      <AuthRequiredModal
        isOpen={true}
        onClose={() => {}}
      />
    );

    expect(screen.getByText('Authentication Required')).toBeDefined();
    expect(screen.getByText('Login required to continue')).toBeDefined();
    expect(screen.getByRole('button', { name: /Continue with Google/i })).toBeDefined();
    expect(screen.getByRole('button', { name: /Continue with GitHub/i })).toBeDefined();
    expect(screen.getByRole('button', { name: /Cancel/i })).toBeDefined();
  });

  it('should render with custom message', () => {
    render(
      <AuthRequiredModal
        isOpen={true}
        onClose={() => {}}
        message="You must be logged in to vote"
      />
    );

    expect(screen.getByText('You must be logged in to vote')).toBeDefined();
  });

  it('should not render when closed', () => {
    const { container } = render(
      <AuthRequiredModal
        isOpen={false}
        onClose={() => {}}
      />
    );

    // Dialog should not be visible
    expect(container.querySelector('[role="dialog"]')).toBeNull();
  });

  it('should store current URL and redirect to Google OAuth on button click', () => {
    const hrefSetter = vi.fn();

    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        ...window.location,
        pathname: '/posts/123',
        get href() { return ''; },
        set href(val: string) { hrefSetter(val); },
      },
    });

    render(
      <AuthRequiredModal
        isOpen={true}
        onClose={() => {}}
      />
    );

    const googleButton = screen.getByRole('button', { name: /Continue with Google/i });
    fireEvent.click(googleButton);

    expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_return_url', '/posts/123');
    expect(hrefSetter).toHaveBeenCalledWith(
      expect.stringContaining('/v1/auth/google')
    );
  });

  it('should store current URL and redirect to GitHub OAuth on button click', () => {
    const hrefSetter = vi.fn();

    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        ...window.location,
        pathname: '/problems/456',
        get href() { return ''; },
        set href(val: string) { hrefSetter(val); },
      },
    });

    render(
      <AuthRequiredModal
        isOpen={true}
        onClose={() => {}}
      />
    );

    const githubButton = screen.getByRole('button', { name: /Continue with GitHub/i });
    fireEvent.click(githubButton);

    expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_return_url', '/problems/456');
    expect(hrefSetter).toHaveBeenCalledWith(
      expect.stringContaining('/v1/auth/github')
    );
  });

  it('should use custom return URL if provided', () => {
    const hrefSetter = vi.fn();

    Object.defineProperty(window, 'location', {
      writable: true,
      value: {
        ...window.location,
        pathname: '/current-page',
        get href() { return ''; },
        set href(val: string) { hrefSetter(val); },
      },
    });

    render(
      <AuthRequiredModal
        isOpen={true}
        onClose={() => {}}
        returnUrl="/custom-return-url"
      />
    );

    const googleButton = screen.getByRole('button', { name: /Continue with Google/i });
    fireEvent.click(googleButton);

    expect(localStorageMock.setItem).toHaveBeenCalledWith('auth_return_url', '/custom-return-url');
  });

  it('should call onClose when cancel button is clicked', () => {
    const onClose = vi.fn();

    render(
      <AuthRequiredModal
        isOpen={true}
        onClose={onClose}
      />
    );

    const cancelButton = screen.getByRole('button', { name: /Cancel/i });
    fireEvent.click(cancelButton);

    expect(onClose).toHaveBeenCalled();
  });
});

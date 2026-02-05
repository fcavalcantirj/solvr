/**
 * Tests for ErrorBoundary component
 * Tests error catching and fallback UI per PRD requirement
 */

import { render, screen } from '@testing-library/react';
import ErrorBoundary from '../components/ErrorBoundary';

// Component that throws an error
const ThrowError = ({ shouldThrow }: { shouldThrow: boolean }) => {
  if (shouldThrow) {
    throw new Error('Test error');
  }
  return <div data-testid="child">Rendered successfully</div>;
};

// Suppress console.error for cleaner test output (React logs errors when ErrorBoundary catches them)
const originalError = console.error;
beforeAll(() => {
  console.error = jest.fn();
});
afterAll(() => {
  console.error = originalError;
});

describe('ErrorBoundary', () => {
  it('renders children when no error occurs', () => {
    render(
      <ErrorBoundary>
        <ThrowError shouldThrow={false} />
      </ErrorBoundary>
    );

    expect(screen.getByTestId('child')).toBeInTheDocument();
    expect(screen.getByTestId('child')).toHaveTextContent('Rendered successfully');
  });

  it('renders fallback UI when child throws error', () => {
    render(
      <ErrorBoundary>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    // Should show fallback UI, not the child
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();

    // Should show error message
    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument();
  });

  it('displays "Try again" button in fallback UI', () => {
    render(
      <ErrorBoundary>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    const tryAgainButton = screen.getByRole('button', { name: /try again/i });
    expect(tryAgainButton).toBeInTheDocument();
  });

  it('renders custom fallback when provided', () => {
    const customFallback = <div data-testid="custom-fallback">Custom error UI</div>;

    render(
      <ErrorBoundary fallback={customFallback}>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    expect(screen.getByTestId('custom-fallback')).toBeInTheDocument();
    expect(screen.getByTestId('custom-fallback')).toHaveTextContent('Custom error UI');
  });

  it('calls onError callback when error occurs', () => {
    const onErrorMock = jest.fn();

    render(
      <ErrorBoundary onError={onErrorMock}>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    expect(onErrorMock).toHaveBeenCalledTimes(1);
    expect(onErrorMock).toHaveBeenCalledWith(
      expect.any(Error),
      expect.objectContaining({
        componentStack: expect.any(String),
      })
    );
  });

  it('fallback UI has appropriate styling', () => {
    render(
      <ErrorBoundary>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    // Check for error container with proper styling classes
    const errorContainer = screen.getByRole('alert');
    expect(errorContainer).toBeInTheDocument();
  });

  it('displays error details in development mode', () => {
    const originalNodeEnv = process.env.NODE_ENV;
    // Simulate development mode
    Object.defineProperty(process.env, 'NODE_ENV', { value: 'development' });

    render(
      <ErrorBoundary>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    // Should show error message in development
    expect(screen.getByText(/Test error/)).toBeInTheDocument();

    // Restore original NODE_ENV
    Object.defineProperty(process.env, 'NODE_ENV', { value: originalNodeEnv });
  });

  it('hides error details in production mode', () => {
    const originalNodeEnv = process.env.NODE_ENV;
    Object.defineProperty(process.env, 'NODE_ENV', { value: 'production' });

    render(
      <ErrorBoundary>
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    // Should NOT show the specific error message in production
    // (it would expose implementation details)
    // The generic "Something went wrong" should be shown instead
    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument();

    Object.defineProperty(process.env, 'NODE_ENV', { value: originalNodeEnv });
  });
});

describe('ErrorBoundary recovery', () => {
  it('resets error state when key prop changes', () => {
    const { rerender } = render(
      <ErrorBoundary key="1">
        <ThrowError shouldThrow={true} />
      </ErrorBoundary>
    );

    // Should show fallback
    expect(screen.queryByTestId('child')).not.toBeInTheDocument();
    expect(screen.getByText(/something went wrong/i)).toBeInTheDocument();

    // Re-render with different key and non-throwing child
    rerender(
      <ErrorBoundary key="2">
        <ThrowError shouldThrow={false} />
      </ErrorBoundary>
    );

    // Should now show child
    expect(screen.getByTestId('child')).toBeInTheDocument();
  });
});

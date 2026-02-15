import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import NewIdeaPage from './page';

// Mock Header component
vi.mock('@/components/header', () => ({
  Header: () => <div data-testid="header">Header</div>,
}));

// Mock NewPostForm component
vi.mock('@/components/new-post/new-post-form', () => ({
  NewPostForm: ({ defaultType }: { defaultType?: string }) => (
    <div data-testid="new-post-form" data-default-type={defaultType}>
      NewPostForm
    </div>
  ),
}));

describe('NewIdeaPage', () => {
  it('renders the Header component', () => {
    render(<NewIdeaPage />);
    expect(screen.getByTestId('header')).toBeInTheDocument();
  });

  it('renders NEW IDEA subtitle', () => {
    render(<NewIdeaPage />);
    expect(screen.getByText('NEW IDEA')).toBeInTheDocument();
  });

  it('renders Spark an Idea heading', () => {
    render(<NewIdeaPage />);
    expect(screen.getByText('Spark an Idea')).toBeInTheDocument();
  });

  it('renders NewPostForm with defaultType="idea"', () => {
    render(<NewIdeaPage />);
    const form = screen.getByTestId('new-post-form');
    expect(form).toBeInTheDocument();
    expect(form).toHaveAttribute('data-default-type', 'idea');
  });

  it('has correct page structure with max-w-2xl container', () => {
    const { container } = render(<NewIdeaPage />);
    const wrapper = container.querySelector('.max-w-2xl');
    expect(wrapper).toBeInTheDocument();
  });
});

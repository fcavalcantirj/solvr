import { render, screen } from '@testing-library/react';
import Home from '../app/page';

// Mock next/image since it requires Next.js context
jest.mock('next/image', () => ({
  __esModule: true,
  default: (props: { alt: string; src: string; [key: string]: unknown }) => {
    // eslint-disable-next-line @next/next/no-img-element
    return <img alt={props.alt} src={props.src} />;
  },
}));

describe('Home Page', () => {
  it('renders the heading', () => {
    render(<Home />);

    const heading = screen.getByRole('heading', { level: 1 });
    expect(heading).toBeInTheDocument();
    expect(heading).toHaveTextContent('To get started, edit the page.tsx file.');
  });

  it('renders the Templates link', () => {
    render(<Home />);

    const templatesLink = screen.getByRole('link', { name: /templates/i });
    expect(templatesLink).toBeInTheDocument();
    expect(templatesLink).toHaveAttribute('href', expect.stringContaining('vercel.com/templates'));
  });

  it('renders the Learning link', () => {
    render(<Home />);

    const learningLink = screen.getByRole('link', { name: /learning/i });
    expect(learningLink).toBeInTheDocument();
    expect(learningLink).toHaveAttribute('href', expect.stringContaining('nextjs.org/learn'));
  });

  it('renders the Deploy Now button', () => {
    render(<Home />);

    const deployLink = screen.getByRole('link', { name: /deploy now/i });
    expect(deployLink).toBeInTheDocument();
    expect(deployLink).toHaveAttribute('target', '_blank');
  });

  it('renders the Documentation link', () => {
    render(<Home />);

    const docsLink = screen.getByRole('link', { name: /documentation/i });
    expect(docsLink).toBeInTheDocument();
    expect(docsLink).toHaveAttribute('href', expect.stringContaining('nextjs.org/docs'));
  });
});

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import ClientCardComment from '@/components/clients/ClientCardComment';

describe('ClientCardComment', () => {
  it('renders a client comment in the card', () => {
    render(<ClientCardComment comment={'Primary mobile client\nLine two'} />);

    const comment = screen.getByText(/Primary mobile client/);
    expect(comment.className).toContain('client-card-comment');
    expect(comment.textContent).toBe('Primary mobile client\nLine two');
    expect(comment.getAttribute('title')).toBe('Primary mobile client\nLine two');
  });

  it('supports the compact desktop style', () => {
    render(<ClientCardComment comment="Desktop comment" className="sub" />);

    expect(screen.getByText('Desktop comment').className).toBe('sub');
  });

  it('renders nothing when no comment is set', () => {
    const { container } = render(<ClientCardComment comment="" />);

    expect(container.childElementCount).toBe(0);
  });
});
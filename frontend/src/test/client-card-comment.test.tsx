import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import ClientCardComment from '@/components/clients/ClientCardComment';

describe('ClientCardComment', () => {
  it('renders a client comment in the card', () => {
    render(<ClientCardComment comment="Primary mobile client" />);

    const comment = screen.getByText('Primary mobile client');
    expect(comment.className).toContain('client-card-comment');
    expect(comment.getAttribute('title')).toBe('Primary mobile client');
  });

  it('renders nothing when no comment is set', () => {
    const { container } = render(<ClientCardComment comment="" />);

    expect(container.childElementCount).toBe(0);
  });
});
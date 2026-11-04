import { expect } from 'chai';
import { resolveTimezone } from '@/lib/timezone';

describe('lib/timezone', () => {
  it('resolves the Local alias to a browser IANA timezone', () => {
    const timezone = resolveTimezone('Local');

    expect(timezone).to.be.a('string').and.not.equal('Local');
    expect(() => new Intl.DateTimeFormat('en-US', { timeZone: timezone }).format()).not.to.throw();
  });

  it('keeps a valid IANA timezone unchanged', () => {
    expect(resolveTimezone('Asia/Shanghai')).to.equal('Asia/Shanghai');
  });

  it('falls back to the browser timezone for an invalid timezone', () => {
    expect(resolveTimezone('Not/AZone')).not.to.equal('Not/AZone');
  });

  it('defaults to a valid timezone when no value is supplied', () => {
    const timezone = resolveTimezone(undefined);

    expect(timezone).to.be.a('string').and.not.equal('');
    expect(() => new Intl.DateTimeFormat('en-US', { timeZone: timezone }).format()).not.to.throw();
  });
});

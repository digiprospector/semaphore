function browserTimezone() {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

export function resolveTimezone(name) {
  if (!name || name === 'Local') {
    return browserTimezone();
  }

  try {
    new Intl.DateTimeFormat('en-US', { timeZone: name }).format();
    return name;
  } catch {
    return browserTimezone();
  }
}

export default resolveTimezone;

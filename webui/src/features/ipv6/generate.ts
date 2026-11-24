const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
const randomBytes = new Uint8Array(16);

export function generatePassword() {
  for (;;) {
    let password = "";
    while (password.length < 8) {
      for (const byte of crypto.getRandomValues(randomBytes)) {
        // Skip values outside a multiple of the alphabet size.
        if (byte < 248) {
          password += alphabet[byte % alphabet.length];
        }
        if (password.length === 8) {
          break;
        }
      }
    }
    if (/[A-Za-z]/.test(password) && /[0-9]/.test(password)) {
      return password;
    }
  }
}

export function ipv6Network(prefix: string): string | null {
  const match = /^([\da-f:]+)\/64$/i.exec(prefix.trim());
  if (!match) {
    return null;
  }
  const halves = match[1].split("::");
  const left = halves[0] ? halves[0].split(":") : [];
  const right = halves[1] ? halves[1].split(":") : [];
  const zeroGroups = 8 - left.length - right.length;
  if (halves.length > 2 || (halves.length === 1 ? zeroGroups !== 0 : zeroGroups < 1)) {
    return null;
  }
  const groups = [...left, ...Array<string>(zeroGroups).fill("0"), ...right];
  if (!groups.every((group) => /^[\da-f]{1,4}$/i.test(group))) {
    return null;
  }
  const numbers = groups.map((group) => parseInt(group, 16));
  if ((numbers[0] & 0xe000) !== 0x2000) {
    return null;
  }
  return numbers.slice(0, 4).map((value) => value.toString(16)).join(":");
}

export function generateAddress(network: string, used: Set<string>) {
  const values = new Uint16Array(4);
  for (;;) {
    crypto.getRandomValues(values);
    if (values.every((value) => value === 0)) {
      continue;
    }
    const address = `${network}:${Array.from(values, (value) => value.toString(16)).join(":")}/64`;
    if (!used.has(address)) {
      used.add(address);
      return address;
    }
  }
}

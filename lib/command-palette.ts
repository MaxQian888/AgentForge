export type PaletteCommandCategory = "navigation" | "actions" | "context";
export type PaletteCommandKind = "navigation" | "action";

export interface PaletteSearchCommand {
  id: string;
  label: string;
  value: string;
  href: string;
  kind: PaletteCommandKind;
  category: PaletteCommandCategory;
  keywords?: string[];
}

function normalizeText(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9\s]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function levenshteinDistance(left: string, right: string) {
  const rows = left.length + 1;
  const columns = right.length + 1;
  const matrix = Array.from({ length: rows }, () => Array<number>(columns).fill(0));

  for (let row = 0; row < rows; row += 1) {
    matrix[row]![0] = row;
  }

  for (let column = 0; column < columns; column += 1) {
    matrix[0]![column] = column;
  }

  for (let row = 1; row < rows; row += 1) {
    for (let column = 1; column < columns; column += 1) {
      const cost = left[row - 1] === right[column - 1] ? 0 : 1;

      matrix[row]![column] = Math.min(
        matrix[row - 1]![column]! + 1,
        matrix[row]![column - 1]! + 1,
        matrix[row - 1]![column - 1]! + cost,
      );
    }
  }

  return matrix[rows - 1]![columns - 1]!;
}

function scoreToken(query: string, token: string) {
  if (!token) {
    return 0;
  }

  if (token === query) {
    return 1_200;
  }

  if (token.startsWith(query)) {
    return 1_000 - (token.length - query.length);
  }

  const containsIndex = token.indexOf(query);
  if (containsIndex >= 0) {
    return 900 - containsIndex;
  }

  const distance = levenshteinDistance(query, token);
  const threshold = query.length >= 5 ? 2 : 1;

  if (distance <= threshold) {
    return 700 - distance * 100;
  }

  return 0;
}

function scoreCommand(query: string, command: PaletteSearchCommand) {
  const normalizedQuery = normalizeText(query);

  if (!normalizedQuery) {
    return 1;
  }

  const searchPool = [command.label, command.value, ...(command.keywords ?? [])]
    .map(normalizeText)
    .filter(Boolean);

  let score = 0;

  for (const entry of searchPool) {
    if (entry === normalizedQuery) {
      score = Math.max(score, 1_300);
      continue;
    }

    const containsIndex = entry.indexOf(normalizedQuery);
    if (containsIndex >= 0) {
      score = Math.max(score, 1_100 - containsIndex);
    }

    for (const token of entry.split(" ")) {
      score = Math.max(score, scoreToken(normalizedQuery, token));
    }
  }

  return score;
}

export function filterPaletteCommands<T extends PaletteSearchCommand>(
  commands: T[],
  query: string,
) {
  const normalizedQuery = normalizeText(query);

  if (!normalizedQuery) {
    return commands;
  }

  return commands
    .map((command) => ({ command, score: scoreCommand(normalizedQuery, command) }))
    .filter((entry) => entry.score > 0)
    .sort(
      (left, right) =>
        right.score - left.score ||
        left.command.label.localeCompare(right.command.label),
    )
    .map((entry) => entry.command);
}

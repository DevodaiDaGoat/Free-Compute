export interface InsertQuery {
  text: string;
  values: unknown[];
}

export function buildInsert(
  table: string,
  data: Record<string, unknown>,
  returning = "*",
): InsertQuery {
  const keys = Object.keys(data);
  const values = Object.values(data);
  const placeholders = keys.map((_, i) => `$${i + 1}`);

  return {
    text: `INSERT INTO ${table} (${keys.join(", ")}) VALUES (${placeholders.join(", ")}) RETURNING ${returning}`,
    values,
  };
}

export function buildUpdate(
  table: string,
  data: Record<string, unknown>,
  whereColumn: string,
  whereValue: unknown,
  returning = "*",
): InsertQuery {
  const keys = Object.keys(data);
  const values = Object.values(data);
  const setClauses = keys.map((key, i) => `${key} = $${i + 1}`);
  values.push(whereValue);

  return {
    text: `UPDATE ${table} SET ${setClauses.join(", ")} WHERE ${whereColumn} = $${values.length} RETURNING ${returning}`,
    values,
  };
}

export interface SelectQuery {
  text: string;
  values: unknown[];
}

export function buildSelect(
  table: string,
  columns = "*",
  where?: Record<string, unknown>,
  options?: { orderBy?: string; limit?: number; offset?: number },
): SelectQuery {
  const values: unknown[] = [];
  let text = `SELECT ${columns} FROM ${table}`;

  if (where && Object.keys(where).length > 0) {
    const conditions = Object.entries(where).map(([key, value], i) => {
      values.push(value);
      return `${key} = $${i + 1}`;
    });
    text += ` WHERE ${conditions.join(" AND ")}`;
  }

  if (options?.orderBy) text += ` ORDER BY ${options.orderBy}`;
  if (options?.limit !== undefined) {
    values.push(options.limit);
    text += ` LIMIT $${values.length}`;
  }
  if (options?.offset !== undefined) {
    values.push(options.offset);
    text += ` OFFSET $${values.length}`;
  }

  return { text, values };
}

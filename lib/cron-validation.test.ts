import { isValidCronExpression, validateCronExpression } from "./cron-validation";

describe("validateCronExpression", () => {
  it.each([
    ["* * * * *"],
    ["*/5 * * * *"],
    ["0 0 * * *"],
    ["0 12 * * 1-5"],
    ["0 0 1 1 *"],
    ["15,45 * * * *"],
    ["0 */2 * * SUN-SAT"],
    ["0 0 * JAN,FEB *"],
    ["0-30/10 * * * *"],
  ])("accepts valid expression %p", (expression) => {
    expect(validateCronExpression(expression)).toBeNull();
    expect(isValidCronExpression(expression)).toBe(true);
  });

  it.each([
    ["", "Cron expression is required"],
    ["   ", "Cron expression is required"],
    ["* * * *", "Expected 5 fields: minute hour day month weekday"],
    ["* * * * * *", "Expected 5 fields: minute hour day month weekday"],
    ["99 * * * *", "minute must be between 0 and 59"],
    ["0 25 * * *", "hour must be between 0 and 23"],
    ["0 0 32 * *", "day-of-month must be between 1 and 31"],
    ["0 0 * 13 *", "month must be between 1 and 12"],
    ["0 0 * * 9", "day-of-week must be between 0 and 7"],
    ["10-5 * * * *", "minute range out of bounds (0-59)"],
    ["*/0 * * * *", "Invalid step in minute"],
    ["abc * * * *", "Invalid minute value"],
  ])("rejects invalid expression %p", (expression, expectedError) => {
    expect(validateCronExpression(expression)).toBe(expectedError);
    expect(isValidCronExpression(expression)).toBe(false);
  });
});

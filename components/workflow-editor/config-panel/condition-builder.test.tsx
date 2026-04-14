import { parseExpression, serializeVisualRule } from "./condition-builder";

describe("condition-builder", () => {
  describe("serializeVisualRule", () => {
    it("serializes a basic comparison", () => {
      const expr = serializeVisualRule("planner", "output.ok", "==", "true");
      expect(expr).toBe("{{planner.output.ok}} == true");
    });

    it("serializes numeric comparison", () => {
      const expr = serializeVisualRule("classify", "output.urgency", ">", "0.7");
      expect(expr).toBe("{{classify.output.urgency}} > 0.7");
    });
  });

  describe("parseExpression", () => {
    it("parses simple template expression", () => {
      const result = parseExpression("{{planner.output.ok}} == true");
      expect(result).toEqual({
        nodeId: "planner",
        field: "output.ok",
        operator: "==",
        value: "true",
      });
    });

    it("returns null for complex expression", () => {
      const result = parseExpression("len({{planner.output.items}}) > 0");
      expect(result).toBeNull();
    });

    it("parses all supported operators", () => {
      for (const op of ["==", "!=", ">", "<", ">=", "<="]) {
        const result = parseExpression(`{{n.output.x}} ${op} 5`);
        expect(result?.operator).toBe(op);
      }
    });
  });
});

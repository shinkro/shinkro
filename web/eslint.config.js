import reactRefresh from "eslint-plugin-react-refresh";
import tseslint from "typescript-eslint";

export default [
  {
    ignores: ["dist", ".eslintrc.cjs"],
  },
  {
    files: ["**/*.js", "**/*.jsx", "**/*.ts", "**/*.tsx"],
    languageOptions: {
      parser: tseslint.parser,
      parserOptions: {
        ecmaVersion: 2020,
        sourceType: "module",
      },
      globals: {
        window: "readonly",
        document: "readonly",
      },
    },
    plugins: {
      "@typescript-eslint": tseslint.plugin,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...tseslint.plugin.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],
    },
  },
  {files:["src/routes.tsx"],rules:{"react-refresh/only-export-components":"off"}},
];

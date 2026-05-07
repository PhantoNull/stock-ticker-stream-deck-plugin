import commonjs from "@rollup/plugin-commonjs";
import resolve from "@rollup/plugin-node-resolve";
import typescript from "@rollup/plugin-typescript";

export default {
  input: "./src/plugin.ts",
  output: {
    file: "./com.exension.stocks.v2.sdPlugin/bin/plugin.js",
    format: "cjs",
    sourcemap: true
  },
  external: [/^node:/],
  plugins: [
    resolve({
      preferBuiltins: true
    }),
    commonjs(),
    typescript({
      tsconfig: "./tsconfig.json"
    })
  ]
};

// node-postgres returns bigint and numeric columns as strings, because they can
// exceed a JS number. Everything Trackline stores in them (counts, minor-unit
// money, coordinates, speeds) is comfortably inside 2^53, and openapi.yaml
// declares them as JSON numbers, so parse them as numbers once, here.
import { types } from 'pg';

types.setTypeParser(types.builtins.INT8, (v) => Number(v));
types.setTypeParser(types.builtins.NUMERIC, (v) => Number(v));

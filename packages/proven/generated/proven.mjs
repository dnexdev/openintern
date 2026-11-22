function word_to_u32(w) {
  let x = 0;
  for (let i = 0; w.$ === "WCon"; i++) {
    x |= Number(w.head) << i;
    w = w.tail;
  }
  return x >>> 0;
}

function u32_to_word(x) {
  let w = {$: "WNil"};
  for (let i = 31; i >= 0; i--) {
    w = {$: "WCon", head: ((x >>> i) & 1) === 1, tail: w};
  }
  return w;
}

function cmp_new(a, b) {
  return {$: a < b ? "LT" : a === b ? "EQ" : "GT"};
}

function nat_divmod(a, b) {
  return b === 0 ? {$: "Tuple", fst: 0, snd: a}
    : {$: "Tuple", fst: Math.trunc(a / b), snd: a % b};
}

function nat_chk(n) {
  if (n > 281474976710655) {
    throw "bend: a Nat past the largest immediate 2^48-1";
  }
  return n;
}

function nat_in(n) {
  if (!Number.isInteger(n) || n < 0 || n > 2 ** 53) {
    throw "bend: expected a Nat host number: an integer from 0 to 2^53";
  }
  return n;
}

function f32_show(x) {
  let s = "nan";
  for (let p = 1; x === x && p <= 9 && f32_round(s) !== x; p += 1) {
    s = String(Number(x.toExponential(p - 1)));
  }
  return (Object.is(x, -0) ? "-0" : s).replace("Infinity", "inf");
}

function f32_bits(x) {
  return new Uint32Array(new Float32Array([x]).buffer)[0];
}

function f32_from_bits(u) {
  return new Float32Array(new Uint32Array([u]).buffer)[0];
}

function f32_read(s) {
  const re = /^[\t\n\v\f\r ]*[+-]?((\d+(?:\.\d*)?|\.\d+)(e[+-]?\d+)?|inf(inity)?|nan)$/i;
  const v = f32_round(s.replace(/inf\w*/i, "Infinity"));
  return re.test(s) ? {$: "Some", value: v} : {$: "None"};
}

const f32_round = function f32_round(s) {
  const d = Number(s);
  const a = Math.abs(d);
  const f = Math.fround(a);
  const g = 2 * a - Math.min(f, 2 ** 128);
  if (g === f || Math.fround(g) !== g || g === Infinity) {
    return Math.sign(d) * f;
  }
  let k = 0;
  while (a * 2 ** k % 1 !== 0) {
    k += 1;
  }
  const [, i, r, e] = /(\d*)\.?(\d*)(?:e([+-]?\d+))?$/i.exec(s);
  const n = Number(e ?? 0) - r.length;
  const x = BigInt(i + r) * 2n ** BigInt(k) * 10n ** BigInt(Math.max(n, 0));
  const y = BigInt(a * 2 ** k) * 10n ** BigInt(Math.max(-n, 0));
  return Math.sign(d) * (x === y || x > y !== g > f ? f : g);
};

const char_new = function char_new(c) {
  return c < 55296 || c >= 57344 && c <= 1114111 ? String.fromCodePoint(c) : String.fromCharCode(56320, c >>> 22, c >>> 11 & 2047, c & 2047);
};

function char_len(s, at = 0) {
  return s[at] === "\uDC00" ? 4 : s.codePointAt(at) > 0xFFFF ? 2 : 1;
}

function char_code(s) {
  return s.length < 4 ? s.codePointAt(0)
    : (s.charCodeAt(1) << 22 | s.charCodeAt(2) << 11 | s.charCodeAt(3)) >>> 0;
}

function string_contains(s, p) {
  // A match must start at a Char boundary, not inside a packed U32 code.
  for (let at = 0; at <= s.length; at += char_len(s, at)) {
    if (s.startsWith(p, at)) {
      return true;
    }
  }
  return false;
}

function str_out(s) {
  return s.replace(/\uDC00[^]{3}/gu, "\uFFFD");
}

function str_in(s) {
  return s.replace(/\p{Cs}/gu, "\uFFFD");
}

// Array
// =====

function array_new(d, v) {
  if (d > 31) {
    throw "bend: an array past the deepest block class 31";
  }
  return Array(2 ** d).fill(v);
}

function array_node(a, b) {
  if (a.length !== b.length) {
    throw "bend: runtime fail-stop";
  }
  return a.concat(b);
}

function array_rmw(a, i, f) {
  const at = i % a.length;
  const old = a[at];
  a[at] = f(old);
  return {$: "Tuple", fst: a, snd: old};
}

// Run
// ===

function run_tail(f, x) {
  return {$: "$JMP", f: f.j?.f === f ? f.j : f, x};
}

function run_clo(j) {
  const f = (x) => run_loop(j(x));
  f.j = j;
  j.f = f;
  return f;
}

function run_loop(r) {
  while (r !== null && typeof r === "object" && r.$ === "$JMP") {
    r = r.f(r.x);
  }
  return r;
}

function run_lib(f, n) {
  return (...a) => a.length < n ? run_lib((...b) => f(...a, ...b), n - a.length)
    : run_loop(f(...a));
}

// Effect
// ======

const $0eff = Object.create(null);

function io_eff(k, run) {
  if (arguments.length > 2) {
    throw new Error("bend: " + k + " takes no need: an effect that waits parks itself");
  }
  if (k in $0eff) {
    throw new Error("bend: two effects register " + k);
  }
  $0eff[k] = run;
}
// Program
// =======

function $packages_proven_bend_rate_limit_ok(_v_0) {
  const _o_0 = _v_0["ok"];
  return _o_0;
}

function $packages_proven_bend_rate_limit_remaining(_v_0) {
  const _r_0 = _v_0["remaining"];
  return _r_0;
}

function $packages_proven_bend_rate_limit_count(_v_0) {
  const _t_0 = _v_0["bucket"];
  const _c_0 = _t_0["count"];
  return _c_0;
}

function $packages_proven_bend_rate_limit_reset_at(_v_0) {
  const _t_0 = _v_0["bucket"];
  const _t_1 = _t_0["reset_at"];
  return _t_1;
}

function $packages_proven_bend_rate_limit_fresh(_now_0, _limit_0, _window_0) {
  if (_limit_0 === 0) {
    return {$: "Verdict", "ok": false, "remaining": 0, "bucket": {$: "Bucket", "count": 0, "reset_at": nat_chk(_now_0 + _window_0)}};
  } else {
    const _l_0 = (_limit_0 - 1);
    return {$: "Verdict", "ok": true, "remaining": _l_0, "bucket": {$: "Bucket", "count": 1, "reset_at": nat_chk(_now_0 + _window_0)}};
  }
}

function $packages_proven_bend_rate_limit_admit_up(_v_0) {
  const _o_0 = _v_0["ok"];
  const _r_0 = _v_0["remaining"];
  const _t_0 = _v_0["bucket"];
  const _c_0 = _t_0["count"];
  const _t_1 = _t_0["reset_at"];
  return {$: "Verdict", "ok": _o_0, "remaining": _r_0, "bucket": {$: "Bucket", "count": nat_chk(_c_0 + 1), "reset_at": _t_1}};
}

function $packages_proven_bend_rate_limit_admit(_count_0, _limit_0, _reset_at_0) {
  if (_count_0 === 0) {
    if (_limit_0 === 0) {
      return {$: "Verdict", "ok": false, "remaining": 0, "bucket": {$: "Bucket", "count": 0, "reset_at": _reset_at_0}};
    } else {
      const _l_0 = (_limit_0 - 1);
      return {$: "Verdict", "ok": true, "remaining": _l_0, "bucket": {$: "Bucket", "count": 1, "reset_at": _reset_at_0}};
    }
  } else {
    const _c_0 = (_count_0 - 1);
    if (_limit_0 === 0) {
      return {$: "Verdict", "ok": false, "remaining": 0, "bucket": {$: "Bucket", "count": nat_chk(_c_0 + 1), "reset_at": _reset_at_0}};
    } else {
      const _l_1 = (_limit_0 - 1);
      return $packages_proven_bend_rate_limit_admit_up(($packages_proven_bend_rate_limit_admit(_c_0, _l_1, _reset_at_0)));
    }
  }
}

function $packages_proven_bend_rate_limit_step_go(_expired_0, _count_0, _reset_at_0, _now_0, _limit_0, _window_0) {
  if (_expired_0) {
    return $packages_proven_bend_rate_limit_fresh(_now_0, _limit_0, _window_0);
  } else {
    return $packages_proven_bend_rate_limit_admit(_count_0, _limit_0, _reset_at_0);
  }
}

function $packages_proven_bend_rate_limit_step(_prev_0, _now_0, _limit_0, _window_0) {
  if (_prev_0.$ === "None") {
    return $packages_proven_bend_rate_limit_fresh(_now_0, _limit_0, _window_0);
  } else {
    const _t_0 = _prev_0["value"];
    const _c_0 = _t_0["count"];
    const _t_1 = _t_0["reset_at"];
    return $packages_proven_bend_rate_limit_step_go(($Nat_is_le(_t_1, _now_0)), _c_0, _t_1, _now_0, _limit_0, _window_0);
  }
}

function $packages_proven_bend_rate_limit_bump(_ok_0, _count_0) {
  if (_ok_0) {
    return nat_chk(_count_0 + 1);
  } else {
    return _count_0;
  }
}

function $packages_proven_bend_freshness_term_end(_t_0) {
  if (_t_0.$ === "Summer") {
    return {$: "MonthDay", "month0": 7, "day": 31};
  } else if (_t_0.$ === "Fall") {
    return {$: "MonthDay", "month0": 11, "day": 31};
  } else if (_t_0.$ === "Winter") {
    return {$: "MonthDay", "month0": 3, "day": 30};
  } else {
    return {$: "MonthDay", "month0": 11, "day": 31};
  }
}

function $packages_proven_bend_freshness_day_ms() {
  return 86400000;
}

function $packages_proven_bend_freshness_max_age_days() {
  return 120;
}

function $packages_proven_bend_freshness_last_seen_days() {
  return 14;
}

function $packages_proven_bend_freshness_any_live(_ends_0, _now_0) {
  if (_ends_0.$ === "Nil") {
    return false;
  } else {
    const _h_0 = _ends_0["head"];
    const _t_0 = _ends_0["tail"];
    const _x_0 = ($Nat_is_le(_now_0, _h_0));
    const _x_1 = ($packages_proven_bend_freshness_any_live(_t_0, _now_0));
    return (_x_0 || _x_1);
  }
}

function $packages_proven_bend_freshness_all_past(_ends_0, _now_0) {
  if (_ends_0.$ === "Nil") {
    return true;
  } else {
    const _h_0 = _ends_0["head"];
    const _t_0 = _ends_0["tail"];
    return $Bool_and((_h_0 < _now_0), ($packages_proven_bend_freshness_all_past(_t_0, _now_0)));
  }
}

function $packages_proven_bend_freshness_is_fresh(_ends_0, _anchor_0, _now_0, _max_ms_0) {
  if (_ends_0.$ === "Nil") {
    return $Nat_is_le(_now_0, nat_chk(_anchor_0 + _max_ms_0));
  } else {
    const _h_0 = _ends_0["head"];
    const _t_0 = _ends_0["tail"];
    return $packages_proven_bend_freshness_any_live({$: "Con", "head": _h_0, "tail": _t_0}, _now_0);
  }
}

function $packages_proven_bend_freshness_is_stale(_ends_0, _anchor_0, _now_0, _max_ms_0) {
  if (_ends_0.$ === "Nil") {
    const _x_0 = nat_chk(_anchor_0 + _max_ms_0);
    return (_x_0 < _now_0);
  } else {
    const _h_0 = _ends_0["head"];
    const _t_0 = _ends_0["tail"];
    return $packages_proven_bend_freshness_all_past({$: "Con", "head": _h_0, "tail": _t_0}, _now_0);
  }
}

function $packages_proven_bend_freshness_is_public(_roles_0, _ends_0, _anchor_0, _now_0, _max_ms_0) {
  if (_roles_0 === 0) {
    return false;
  } else {
    return $packages_proven_bend_freshness_is_fresh(_ends_0, _anchor_0, _now_0, _max_ms_0);
  }
}

function $packages_proven_bend_freshness_stale_last_seen(_seen_0, _now_0, _max_ms_0) {
  const _x_0 = nat_chk(_seen_0 + _max_ms_0);
  return (_x_0 < _now_0);
}

function $packages_proven_bend_apply_picker_needs_picker(_postings_0, _locations_0) {
  if (_postings_0 === 0) {
    return false;
  } else if (_postings_0 === 1) {
    return $Nat_is_le(2, _locations_0);
  } else {
    return true;
  }
}

function $packages_proven_bend_apply_picker_qualify_single_title(_locations_0) {
  return $Nat_is_le(2, _locations_0);
}

function $packages_proven_bend_apply_picker_says_openings(_postings_0, _unique_locations_0) {
  return (_unique_locations_0 < _postings_0);
}

function $packages_proven_bend_deactivation_policy(_fetch_ok_0, _fetched_0, _tech_0) {
  if (!_fetch_ok_0) {
    return {$: "Keep"};
  } else {
    if (_fetched_0 === 0) {
      if (_tech_0 === 0) {
        return {$: "All"};
      } else {
        return {$: "Missing"};
      }
    } else {
      if (_tech_0 === 0) {
        return {$: "Keep"};
      } else {
        return {$: "Missing"};
      }
    }
  }
}

function $packages_proven_bend_funnel_or0(_m_0) {
  if (_m_0.$ === "None") {
    return 0;
  } else {
    const _n_0 = _m_0["value"];
    return _n_0;
  }
}

function $packages_proven_bend_funnel_zero() {
  return {$: "Totals", "fetched": 0, "title": 0, "tech": 0, "upserted": 0, "desc_missing": 0};
}

function $packages_proven_bend_funnel_of(_f_0) {
  const _a_0 = _f_0["fetched"];
  const _b_0 = _f_0["title"];
  const _c_0 = _f_0["tech"];
  const _d_0 = _f_0["upserted"];
  const _e_0 = _f_0["desc_missing"];
  return {$: "Totals", "fetched": ($packages_proven_bend_funnel_or0(_a_0)), "title": ($packages_proven_bend_funnel_or0(_b_0)), "tech": ($packages_proven_bend_funnel_or0(_c_0)), "upserted": ($packages_proven_bend_funnel_or0(_d_0)), "desc_missing": ($packages_proven_bend_funnel_or0(_e_0))};
}

function $packages_proven_bend_funnel_plus(_x_0, _y_0) {
  const _a_0 = _x_0["fetched"];
  const _b_0 = _x_0["title"];
  const _c_0 = _x_0["tech"];
  const _d_0 = _x_0["upserted"];
  const _e_0 = _x_0["desc_missing"];
  const _p_0 = _y_0["fetched"];
  const _q_0 = _y_0["title"];
  const _r_0 = _y_0["tech"];
  const _s_0 = _y_0["upserted"];
  const _t_0 = _y_0["desc_missing"];
  return {$: "Totals", "fetched": nat_chk(_a_0 + _p_0), "title": nat_chk(_b_0 + _q_0), "tech": nat_chk(_c_0 + _r_0), "upserted": nat_chk(_d_0 + _s_0), "desc_missing": nat_chk(_e_0 + _t_0)};
}

function $packages_proven_bend_funnel_total(_fs_0) {
  if (_fs_0.$ === "Nil") {
    return $packages_proven_bend_funnel_zero();
  } else {
    const _f_0 = _fs_0["head"];
    const _t_0 = _fs_0["tail"];
    return $packages_proven_bend_funnel_plus(($packages_proven_bend_funnel_of(_f_0)), ($packages_proven_bend_funnel_total(_t_0)));
  }
}

function $packages_proven_bend_funnel_zero_match_tag() {
  return "zero_match:";
}

function $packages_proven_bend_funnel_is_zero_match(_error_0) {
  return $String_starts_with(_error_0, ($packages_proven_bend_funnel_zero_match_tag()));
}

function $packages_proven_bend_pagination_pred_or(_m_0, _fallback_0) {
  if (_m_0.$ === "None") {
    return _fallback_0;
  } else {
    const _t_0 = _m_0["value"];
    if (_t_0 === 0) {
      return _fallback_0;
    } else {
      const _p_0 = (_t_0 - 1);
      return _p_0;
    }
  }
}

function $packages_proven_bend_pagination_parse(_raw_0) {
  if (_raw_0.$ === "None") {
    return {$: "None"};
  } else {
    const _s_0 = _raw_0["value"];
    return $Nat_read(_s_0);
  }
}

function $packages_proven_bend_pagination_clamp(_raw_0, _default_pred_0, _max_pred_0) {
  const _x_0 = ($packages_proven_bend_pagination_pred_or(($packages_proven_bend_pagination_parse(_raw_0)), _default_pred_0));
  return nat_chk((_max_pred_0 < _x_0 ? _max_pred_0 : _x_0) + 1);
}

function $packages_proven_bend_pagination_page(_raw_0) {
  return $packages_proven_bend_pagination_clamp(_raw_0, 0, 99999);
}

function $packages_proven_bend_pagination_limit(_raw_0) {
  return $packages_proven_bend_pagination_clamp(_raw_0, 26, 99);
}

function $packages_proven_bend_pagination_offset(_page_0, _limit_0) {
  if (_page_0 === 0) {
    return 0;
  } else {
    const _p_0 = (_page_0 - 1);
    return nat_chk(_p_0 * _limit_0);
  }
}

function $packages_proven_bend_job_slug_hex_char(_x_0) {
  if (_x_0.$ === "X0") {
    return "0";
  } else if (_x_0.$ === "X1") {
    return "1";
  } else if (_x_0.$ === "X2") {
    return "2";
  } else if (_x_0.$ === "X3") {
    return "3";
  } else if (_x_0.$ === "X4") {
    return "4";
  } else if (_x_0.$ === "X5") {
    return "5";
  } else if (_x_0.$ === "X6") {
    return "6";
  } else if (_x_0.$ === "X7") {
    return "7";
  } else if (_x_0.$ === "X8") {
    return "8";
  } else if (_x_0.$ === "X9") {
    return "9";
  } else if (_x_0.$ === "Xa") {
    return "a";
  } else if (_x_0.$ === "Xb") {
    return "b";
  } else if (_x_0.$ === "Xc") {
    return "c";
  } else if (_x_0.$ === "Xd") {
    return "d";
  } else if (_x_0.$ === "Xe") {
    return "e";
  } else {
    return "f";
  }
}

function $packages_proven_bend_job_slug_hex_str(_xs_0) {
  if (_xs_0.$ === "Nil") {
    return "";
  } else {
    const _x_0 = _xs_0["head"];
    const _t_0 = _xs_0["tail"];
    return (($packages_proven_bend_job_slug_hex_char(_x_0)) + ($packages_proven_bend_job_slug_hex_str(_t_0)));
  }
}

function $packages_proven_bend_job_slug_hex8_str(_x_0) {
  const _a_0 = _x_0["a"];
  const _b_0 = _x_0["b"];
  const _c_0 = _x_0["c"];
  const _d_0 = _x_0["d"];
  const _e_0 = _x_0["e"];
  const _f_0 = _x_0["f"];
  const _g_0 = _x_0["g"];
  const _h_0 = _x_0["h"];
  return $packages_proven_bend_job_slug_hex_str({$: "Con", "head": _a_0, "tail": {$: "Con", "head": _b_0, "tail": {$: "Con", "head": _c_0, "tail": {$: "Con", "head": _d_0, "tail": {$: "Con", "head": _e_0, "tail": {$: "Con", "head": _f_0, "tail": {$: "Con", "head": _g_0, "tail": {$: "Con", "head": _h_0, "tail": {$: "Nil"}}}}}}}}});
}

function $packages_proven_bend_job_slug_le_f(_c_0) {
  const _x_0 = char_code(_c_0);
  return (_x_0 <= 102);
}

function $packages_proven_bend_job_slug_is_hex(_c_0) {
  const _l_0 = ($Char_to_lower(_c_0));
  const _x_0 = ($Char_is_digit(_c_0));
  const _x_1 = ($Bool_and(($Char_is_lower(_l_0)), ($packages_proven_bend_job_slug_le_f(_l_0))));
  return (_x_0 || _x_1);
}

function $packages_proven_bend_job_slug_hex_digits_if(_ok_0, _c_0, _rest_0) {
  if (!_ok_0) {
    return {$: "None"};
  } else {
    if (_rest_0.$ === "None") {
      return {$: "None"};
    } else {
      const _r_0 = _rest_0["value"];
      return {$: "Some", "value": (_c_0 + _r_0)};
    }
  }
}

function $packages_proven_bend_job_slug_hex_digits(_s_0) {
  if (_s_0 === "") {
    return {$: "Some", "value": ""};
  } else {
    const _c_0 = _s_0.slice(0, char_len(_s_0));
    const _t_0 = _s_0.slice(char_len(_s_0));
    return $packages_proven_bend_job_slug_hex_digits_if(($packages_proven_bend_job_slug_is_hex(_c_0)), ($Char_to_lower(_c_0)), ($packages_proven_bend_job_slug_hex_digits(_t_0)));
  }
}

function $packages_proven_bend_job_slug_strip_dashes_if(_dash_0, _c_0, _rest_0) {
  if (_dash_0) {
    return _rest_0;
  } else {
    return (_c_0 + _rest_0);
  }
}

function $packages_proven_bend_job_slug_strip_dashes(_s_0) {
  if (_s_0 === "") {
    return "";
  } else {
    const _c_0 = _s_0.slice(0, char_len(_s_0));
    const _t_0 = _s_0.slice(char_len(_s_0));
    return $packages_proven_bend_job_slug_strip_dashes_if(($Char_is_eq(_c_0, "-")), _c_0, ($packages_proven_bend_job_slug_strip_dashes(_t_0)));
  }
}

function $packages_proven_bend_job_slug_suffix(_id_0) {
  return $String_take(($packages_proven_bend_job_slug_strip_dashes(_id_0)), 8);
}

function $packages_proven_bend_job_slug_segment(_base_0, _id_0) {
  const _x_0 = ($packages_proven_bend_job_slug_suffix(_id_0));
  const _x_1 = ("-" + _x_0);
  return (_base_0 + _x_1);
}

function $packages_proven_bend_job_slug_parse_dash(_dash_0, _len8_0, _digits_0) {
  if (_dash_0) {
    if (_len8_0) {
      return _digits_0;
    } else {
      return {$: "None"};
    }
  } else {
    return {$: "None"};
  }
}

function $packages_proven_bend_job_slug_parse_tail(_s_0) {
  if (_s_0 === "") {
    return {$: "None"};
  } else {
    const _c_0 = _s_0.slice(0, char_len(_s_0));
    const _r_0 = _s_0.slice(char_len(_s_0));
    return $packages_proven_bend_job_slug_parse_dash(($Char_is_eq(_c_0, "-")), ($Nat_is_eq([...str_out(_r_0)].length, 8)), ($packages_proven_bend_job_slug_hex_digits(_r_0)));
  }
}

function $packages_proven_bend_job_slug_parse(_slug_0) {
  const _x_0 = [...str_out(_slug_0)].length;
  return $packages_proven_bend_job_slug_parse_tail(($String_drop(_slug_0, (_x_0 < 9 ? 0 : _x_0 - 9))));
}

function $LAWS_rl_remaining(_prev_0, _now_0, _limit_0, _window_0) {
  if (_prev_0.$ === "None") {
    return $rl_fresh_rem(_now_0, _limit_0, _window_0);
  } else {
    const _t_0 = _prev_0["value"];
    const _c_0 = _t_0["count"];
    const _t_1 = _t_0["reset_at"];
    return $rl_go_rem(($Nat_is_le(_t_1, _now_0)), _c_0, _t_1, _now_0, _limit_0, _window_0);
  }
}

function $LAWS_rl_bounded(_prev_0, _now_0, _limit_0, _window_0) {
  if (_prev_0.$ === "None") {
    return $rl_fresh_bounded(_now_0, _limit_0, _window_0);
  } else {
    const _t_0 = _prev_0["value"];
    const _c_0 = _t_0["count"];
    const _t_1 = _t_0["reset_at"];
    return $rl_go_bounded(($Nat_is_le(_t_1, _now_0)), _c_0, _t_1, _now_0, _limit_0, _window_0);
  }
}

function $LAWS_rl_zero_limit(_prev_0, _now_0, _window_0) {
  if (_prev_0.$ === "None") {
    return null;
  } else {
    const _t_0 = _prev_0["value"];
    const _c_0 = _t_0["count"];
    const _t_1 = _t_0["reset_at"];
    return $rl_go_zero(($Nat_is_le(_t_1, _now_0)), _c_0, _t_1, _now_0, _window_0);
  }
}

function $LAWS_rl_expired(_count_0, _reset_at_0, _k_0, _limit_0, _window_0) {
  return null;
}

function $LAWS_rl_open_window(_count_0, _now_0, _k_0, _limit_0, _window_0) {
  return null;
}

function $LAWS_rl_admit_count(_count_0, _limit_0, _reset_at_0) {
  return $rl_admit_count(_count_0, _limit_0, _reset_at_0);
}

function $LAWS_rl_admit_keeps_window(_count_0, _limit_0, _reset_at_0) {
  return $rl_admit_reset(_count_0, _limit_0, _reset_at_0);
}

function $LAWS_rl_fresh_window(_now_0, _limit_0, _window_0) {
  if (_limit_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $LAWS_fr_stale_iff_not_fresh(_ends_0, _anchor_0, _now_0, _max_ms_0) {
  if (_ends_0.$ === "Nil") {
    return $packages_proven_bend_lemmas_lt_not_le(nat_chk(_anchor_0 + _max_ms_0), _now_0);
  } else {
    const _h_0 = _ends_0["head"];
    const _t_0 = _ends_0["tail"];
    return $fr_past_live({$: "Con", "head": _h_0, "tail": _t_0}, _now_0);
  }
}

function $LAWS_fr_stale_monotone(_ends_0, _anchor_0, _now_0, _k_0, _max_ms_0, _e_0) {
  if (_ends_0.$ === "Nil") {
    return $packages_proven_bend_lemmas_lt_mono(nat_chk(_anchor_0 + _max_ms_0), _now_0, _k_0, _e_0);
  } else {
    const _h_0 = _ends_0["head"];
    const _t_0 = _ends_0["tail"];
    return $fr_all_past_mono({$: "Con", "head": _h_0, "tail": _t_0}, _now_0, _k_0, _e_0);
  }
}

function $LAWS_fr_last_seen_monotone(_seen_0, _now_0, _k_0, _max_ms_0, _e_0) {
  return $packages_proven_bend_lemmas_lt_mono(nat_chk(_seen_0 + _max_ms_0), _now_0, _k_0, _e_0);
}

function $LAWS_fr_term_live_at_end(_h_0, _t_0, _anchor_0, _max_ms_0) {
  return null;
}

function $LAWS_fr_age_live_at_limit(_anchor_0, _max_ms_0) {
  return $packages_proven_bend_lemmas_le_refl(nat_chk(_anchor_0 + _max_ms_0));
}

function $LAWS_fr_public_needs_role(_ends_0, _anchor_0, _now_0, _max_ms_0) {
  return null;
}

function $LAWS_fr_public_is_fresh(_roles_0, _ends_0, _anchor_0, _now_0, _max_ms_0) {
  return null;
}

function $LAWS_fr_summer_end() {
  return null;
}

function $LAWS_fr_fall_end() {
  return null;
}

function $LAWS_fr_winter_end() {
  return null;
}

function $LAWS_fr_other_end() {
  return null;
}

function $LAWS_ap_empty(_locations_0) {
  return null;
}

function $LAWS_ap_many_postings(_p_0, _locations_0) {
  return null;
}

function $LAWS_ap_one_posting(_locations_0) {
  return null;
}

function $LAWS_ap_title_matches_picker(_locations_0) {
  return null;
}

function $LAWS_ap_distinct_says_locations(_n_0) {
  return $packages_proven_bend_lemmas_lt_irrefl(_n_0);
}

function $LAWS_ap_repeats_say_openings(_unique_0, _k_0) {
  return $packages_proven_bend_lemmas_lt_succ_add(_unique_0, _k_0);
}

function $LAWS_dp_failed_fetch_keeps(_fetched_0, _tech_0) {
  return null;
}

function $LAWS_dp_matches_retire_missing(_fetched_0, _tech_0) {
  if (_fetched_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $LAWS_dp_empty_board_retires_all() {
  return null;
}

function $LAWS_dp_zero_match_keeps(_fetched_0) {
  return null;
}

function $LAWS_fn_total_empty() {
  return null;
}

function $LAWS_fn_total_append(_xs_0, _ys_0) {
  if (_xs_0.$ === "Nil") {
    return $fn_zero_plus(($packages_proven_bend_funnel_total(_ys_0)));
  } else {
    return null;
  }
}

function $LAWS_fn_zero_match_tagged(_rest_0) {
  return null;
}

function $LAWS_fn_funnel_not_zero_match(_rest_0) {
  return null;
}

function $LAWS_pg_page_min(_raw_0) {
  const _x_0 = ($packages_proven_bend_pagination_pred_or(($packages_proven_bend_pagination_parse(_raw_0)), 0));
  return $packages_proven_bend_lemmas_le_one_succ((99999 < _x_0 ? 99999 : _x_0));
}

function $LAWS_pg_page_max(_raw_0) {
  return $packages_proven_bend_lemmas_min_le_l(99999, ($packages_proven_bend_pagination_pred_or(($packages_proven_bend_pagination_parse(_raw_0)), 0)));
}

function $LAWS_pg_limit_min(_raw_0) {
  const _x_0 = ($packages_proven_bend_pagination_pred_or(($packages_proven_bend_pagination_parse(_raw_0)), 26));
  return $packages_proven_bend_lemmas_le_one_succ((99 < _x_0 ? 99 : _x_0));
}

function $LAWS_pg_limit_max(_raw_0) {
  return $packages_proven_bend_lemmas_min_le_l(99, ($packages_proven_bend_pagination_pred_or(($packages_proven_bend_pagination_parse(_raw_0)), 26)));
}

function $LAWS_pg_defaults() {
  return null;
}

function $LAWS_pg_first_page_offset(_limit_0) {
  return null;
}

function $LAWS_js_roundtrip(_base_0, _id_0) {
  const _a_0 = _id_0["a"];
  const _b_0 = _id_0["b"];
  const _c_0 = _id_0["c"];
  const _d_0 = _id_0["d"];
  const _e_0 = _id_0["e"];
  const _f_0 = _id_0["f"];
  const _g_0 = _id_0["g"];
  const _h_0 = _id_0["h"];
  const _hs_0 = {$: "Con", "head": _a_0, "tail": {$: "Con", "head": _b_0, "tail": {$: "Con", "head": _c_0, "tail": {$: "Con", "head": _d_0, "tail": {$: "Con", "head": _e_0, "tail": {$: "Con", "head": _f_0, "tail": {$: "Con", "head": _g_0, "tail": {$: "Con", "head": _h_0, "tail": {$: "Nil"}}}}}}}}};
  return $js_digits_hex(_hs_0);
}

function $packages_proven_bend_lemmas_TrueDisc(_b_0) {
  if (_b_0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_ft_empty(_e_0) {
  return {$: "Unit"};
}

function $packages_proven_bend_lemmas_empty_elim(_x_0) {
  throw "bend: runtime fail-stop";
}

function $packages_proven_bend_lemmas_ft(_e_0) {
  return $packages_proven_bend_lemmas_empty_elim(($packages_proven_bend_lemmas_ft_empty(_e_0)));
}

function $packages_proven_bend_lemmas_not_or(_a_0, _b_0) {
  if (_a_0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_not_not(_a_0) {
  if (_a_0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_le_refl($0) {
  for (;;) {
    {
      const _a_0 = $0;
      if (_a_0 === 0) {
        return null;
      } else {
        const _p_0 = (_a_0 - 1);
        $0 = _p_0;
        continue;
      }
    }
  }
}

function $packages_proven_bend_lemmas_le_add($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _k_0 = $1;
      if (_a_0 === 0) {
        if (_k_0 === 0) {
          return null;
        } else {
          return null;
        }
      } else {
        const _p_0 = (_a_0 - 1);
        $0 = _p_0;
        $1 = _k_0;
        continue;
      }
    }
  }
}

function $packages_proven_bend_lemmas_lt_add_succ($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _k_0 = $1;
      if (_a_0 === 0) {
        return null;
      } else {
        const _p_0 = (_a_0 - 1);
        $0 = _p_0;
        $1 = _k_0;
        continue;
      }
    }
  }
}

function $packages_proven_bend_lemmas_lt_not_le($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _b_0 = $1;
      if (_a_0 === 0) {
        if (_b_0 === 0) {
          return null;
        } else {
          return null;
        }
      } else {
        const _p_0 = (_a_0 - 1);
        if (_b_0 === 0) {
          return null;
        } else {
          const _q_1 = (_b_0 - 1);
          $0 = _p_0;
          $1 = _q_1;
          continue;
        }
      }
    }
  }
}

function $packages_proven_bend_lemmas_add_zero(_a_0) {
  if (_a_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_add_succ(_a_0, _b_0) {
  if (_a_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_add_assoc(_a_0, _b_0, _c_0) {
  if (_a_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_sub_zero(_a_0) {
  if (_a_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_add_sub($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _b_0 = $1;
      if (_b_0 === 0) {
        return $Equal_trans(($packages_proven_bend_lemmas_sub_zero(nat_chk(_a_0 + 0))), ($packages_proven_bend_lemmas_add_zero(_a_0)));
      } else {
        const _q_0 = (_b_0 - 1);
        $0 = _a_0;
        $1 = _q_0;
        continue;
      }
    }
  }
}

function $packages_proven_bend_lemmas_and_l(_a_0, _b_0, _e_0) {
  if (_a_0) {
    return null;
  } else {
    return $packages_proven_bend_lemmas_ft(_e_0);
  }
}

function $packages_proven_bend_lemmas_and_r(_a_0, _b_0, _e_0) {
  if (_a_0) {
    return _e_0;
  } else {
    return $packages_proven_bend_lemmas_ft(_e_0);
  }
}

function $packages_proven_bend_lemmas_and_intro(_a_0, _b_0, _ea_0, _eb_0) {
  return _eb_0;
}

function $packages_proven_bend_lemmas_lt_mono($0, $1, $2, $3) {
  for (;;) {
    {
      const _x_0 = $0;
      const _n_0 = $1;
      const _k_0 = $2;
      const _e_0 = $3;
      if (_x_0 === 0) {
        if (_n_0 === 0) {
          return $packages_proven_bend_lemmas_ft(_e_0);
        } else {
          return null;
        }
      } else {
        const _p_0 = (_x_0 - 1);
        if (_n_0 === 0) {
          return $packages_proven_bend_lemmas_ft(_e_0);
        } else {
          const _q_1 = (_n_0 - 1);
          $0 = _p_0;
          $1 = _q_1;
          $2 = _k_0;
          $3 = _e_0;
          continue;
        }
      }
    }
  }
}

function $packages_proven_bend_lemmas_lt_irrefl($0) {
  for (;;) {
    {
      const _a_0 = $0;
      if (_a_0 === 0) {
        return null;
      } else {
        const _p_0 = (_a_0 - 1);
        $0 = _p_0;
        continue;
      }
    }
  }
}

function $packages_proven_bend_lemmas_lt_succ_add($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _k_0 = $1;
      if (_a_0 === 0) {
        return null;
      } else {
        const _p_0 = (_a_0 - 1);
        $0 = _p_0;
        $1 = _k_0;
        continue;
      }
    }
  }
}

function $packages_proven_bend_lemmas_le_one_succ(_m_0) {
  if (_m_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $packages_proven_bend_lemmas_min_le_l($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _b_0 = $1;
      if (_a_0 === 0) {
        return null;
      } else {
        const _p_0 = (_a_0 - 1);
        if (_b_0 === 0) {
          return null;
        } else {
          const _q_0 = (_b_0 - 1);
          $0 = _p_0;
          $1 = _q_0;
          continue;
        }
      }
    }
  }
}

function $rl_up_ok(_v_0) {
  const _t_0 = _v_0["bucket"];
  return null;
}

function $rl_up_rem(_v_0) {
  const _t_0 = _v_0["bucket"];
  return null;
}

function $rl_up_count(_v_0) {
  const _t_0 = _v_0["bucket"];
  return null;
}

function $rl_up_reset(_v_0) {
  const _t_0 = _v_0["bucket"];
  return null;
}

function $rl_bump_succ(_b_0, _n_0) {
  if (_b_0) {
    return null;
  } else {
    return null;
  }
}

function $rl_admit_rem($0, $1, $2) {
  for (;;) {
    {
      const _c_0 = $0;
      const _l_0 = $1;
      const _r_0 = $2;
      if (_c_0 === 0) {
        if (_l_0 === 0) {
          return null;
        } else {
          const _m_0 = (_l_0 - 1);
          return $Equal_sym(($packages_proven_bend_lemmas_sub_zero(_m_0)));
        }
      } else {
        const _d_0 = (_c_0 - 1);
        if (_l_0 === 0) {
          return null;
        } else {
          const _m_1 = (_l_0 - 1);
          $0 = _d_0;
          $1 = _m_1;
          $2 = _r_0;
          continue;
        }
      }
    }
  }
}

function $rl_admit_bounded($0, $1, $2) {
  for (;;) {
    {
      const _c_0 = $0;
      const _l_0 = $1;
      const _r_0 = $2;
      if (_c_0 === 0) {
        if (_l_0 === 0) {
          return null;
        } else {
          const _m_0 = (_l_0 - 1);
          return $packages_proven_bend_lemmas_le_one_succ(_m_0);
        }
      } else {
        const _d_0 = (_c_0 - 1);
        if (_l_0 === 0) {
          return null;
        } else {
          const _m_1 = (_l_0 - 1);
          $0 = _d_0;
          $1 = _m_1;
          $2 = _r_0;
          continue;
        }
      }
    }
  }
}

function $rl_admit_zero(_c_0, _r_0) {
  if (_c_0 === 0) {
    return null;
  } else {
    return null;
  }
}

function $rl_admit_count(_c_0, _l_0, _r_0) {
  if (_c_0 === 0) {
    if (_l_0 === 0) {
      return null;
    } else {
      return null;
    }
  } else {
    if (_l_0 === 0) {
      return null;
    } else {
      return null;
    }
  }
}

function $rl_admit_reset($0, $1, $2) {
  for (;;) {
    {
      const _c_0 = $0;
      const _l_0 = $1;
      const _r_0 = $2;
      if (_c_0 === 0) {
        if (_l_0 === 0) {
          return null;
        } else {
          return null;
        }
      } else {
        const _d_0 = (_c_0 - 1);
        if (_l_0 === 0) {
          return null;
        } else {
          const _m_1 = (_l_0 - 1);
          $0 = _d_0;
          $1 = _m_1;
          $2 = _r_0;
          continue;
        }
      }
    }
  }
}

function $rl_fresh_rem(_now_0, _l_0, _w_0) {
  if (_l_0 === 0) {
    return null;
  } else {
    const _m_0 = (_l_0 - 1);
    return $Equal_sym(($packages_proven_bend_lemmas_sub_zero(_m_0)));
  }
}

function $rl_fresh_bounded(_now_0, _l_0, _w_0) {
  if (_l_0 === 0) {
    return null;
  } else {
    const _m_0 = (_l_0 - 1);
    return $packages_proven_bend_lemmas_le_one_succ(_m_0);
  }
}

function $rl_go_rem(_e_0, _c_0, _r_0, _now_0, _l_0, _w_0) {
  if (_e_0) {
    return $rl_fresh_rem(_now_0, _l_0, _w_0);
  } else {
    return $rl_admit_rem(_c_0, _l_0, _r_0);
  }
}

function $rl_go_bounded(_e_0, _c_0, _r_0, _now_0, _l_0, _w_0) {
  if (_e_0) {
    return $rl_fresh_bounded(_now_0, _l_0, _w_0);
  } else {
    return $rl_admit_bounded(_c_0, _l_0, _r_0);
  }
}

function $rl_go_zero(_e_0, _c_0, _r_0, _now_0, _w_0) {
  if (_e_0) {
    return null;
  } else {
    return $rl_admit_zero(_c_0, _r_0);
  }
}

function $fr_past_live(_xs_0, _n_0) {
  if (_xs_0.$ === "Nil") {
    return null;
  } else {
    return null;
  }
}

function $fr_all_past_mono(_xs_0, _n_0, _k_0, _e_0) {
  if (_xs_0.$ === "Nil") {
    return null;
  } else {
    const _h_0 = _xs_0["head"];
    const _t_0 = _xs_0["tail"];
    const _x_0 = nat_chk(_n_0 + _k_0);
    return $packages_proven_bend_lemmas_and_intro((_h_0 < _x_0), ($packages_proven_bend_freshness_all_past(_t_0, nat_chk(_n_0 + _k_0))), ($packages_proven_bend_lemmas_lt_mono(_h_0, _n_0, _k_0, ($packages_proven_bend_lemmas_and_l((_h_0 < _n_0), ($packages_proven_bend_freshness_all_past(_t_0, _n_0)), _e_0)))), ($fr_all_past_mono(_t_0, _n_0, _k_0, ($packages_proven_bend_lemmas_and_r((_h_0 < _n_0), ($packages_proven_bend_freshness_all_past(_t_0, _n_0)), _e_0)))));
  }
}

function $fn_zero_plus(_t_0) {
  return null;
}

function $fn_plus_assoc(_x_0, _y_0, _z_0) {
  return null;
}

function $js_len_append(_a_0, _b_0) {
  if (_a_0 === "") {
    return null;
  } else {
    return null;
  }
}

function $js_drop_append($0, $1) {
  for (;;) {
    {
      const _a_0 = $0;
      const _b_0 = $1;
      if (_a_0 === "") {
        if (_b_0 === "") {
          return null;
        } else {
          return null;
        }
      } else {
        const _t_1 = _a_0.slice(char_len(_a_0));
        $0 = _t_1;
        $1 = _b_0;
        continue;
      }
    }
  }
}

function $js_step(_x_0, _r_0) {
  if (_x_0.$ === "X0") {
    return null;
  } else if (_x_0.$ === "X1") {
    return null;
  } else if (_x_0.$ === "X2") {
    return null;
  } else if (_x_0.$ === "X3") {
    return null;
  } else if (_x_0.$ === "X4") {
    return null;
  } else if (_x_0.$ === "X5") {
    return null;
  } else if (_x_0.$ === "X6") {
    return null;
  } else if (_x_0.$ === "X7") {
    return null;
  } else if (_x_0.$ === "X8") {
    return null;
  } else if (_x_0.$ === "X9") {
    return null;
  } else if (_x_0.$ === "Xa") {
    return null;
  } else if (_x_0.$ === "Xb") {
    return null;
  } else if (_x_0.$ === "Xc") {
    return null;
  } else if (_x_0.$ === "Xd") {
    return null;
  } else if (_x_0.$ === "Xe") {
    return null;
  } else {
    return null;
  }
}

function $js_digits_hex(_xs_0) {
  if (_xs_0.$ === "Nil") {
    return null;
  } else {
    const _x_0 = _xs_0["head"];
    const _t_0 = _xs_0["tail"];
    return $js_step(_x_0, ($packages_proven_bend_job_slug_hex_str(_t_0)));
  }
}

function $Nat_is_le(_a_0, _b_0) {
  return $Cmp_is_le(cmp_new(_a_0, _b_0));
}

function $Bool_and(_a_0, _b_0) {
  if (!_a_0) {
    return false;
  } else {
    return _b_0;
  }
}

function $String_starts_with(_s_0, _p_0) {
  return $String_starts_with_go(_p_0, _s_0);
}

function $Nat_read(_s_0) {
  if (_s_0 === "") {
    return {$: "None"};
  } else {
    const _h_0 = _s_0.slice(0, char_len(_s_0));
    const _t_0 = _s_0.slice(char_len(_s_0));
    const _u_0 = ($Nat_read_trim((_h_0 + _t_0)));
    return $Nat_read_gate(_u_0, ($Nat_read_fits(_u_0)));
  }
}

function $Char_to_lower(_c_0) {
  const _x_0 = ($Bool_to_u32(($Char_is_upper(_c_0))));
  const _x_1 = char_code(_c_0);
  const _x_2 = (Math.imul(_x_0, 32) >>> 0);
  return char_new(((_x_1 + _x_2) >>> 0));
}

function $Char_is_digit(_c_0) {
  const _x_0 = char_code(_c_0);
  const _x_1 = char_code(_c_0);
  return $Bool_and((_x_0 >= 48), (_x_1 <= 57));
}

function $Char_is_lower(_c_0) {
  const _x_0 = char_code(_c_0);
  const _x_1 = char_code(_c_0);
  return $Bool_and((_x_0 >= 97), (_x_1 <= 122));
}

function $Char_is_eq(_a_0, _b_0) {
  const _x_0 = char_code(_a_0);
  const _x_1 = char_code(_b_0);
  return (_x_0 === _x_1);
}

function $String_take(_s_0, _n_0) {
  if (_s_0 === "") {
    return "";
  } else {
    const _h_0 = _s_0.slice(0, char_len(_s_0));
    const _t_0 = _s_0.slice(char_len(_s_0));
    if (_n_0 === 0) {
      return "";
    } else {
      const _p_0 = (_n_0 - 1);
      return (_h_0 + ($String_take(_t_0, _p_0)));
    }
  }
}

function $Nat_is_eq(_a_0, _b_0) {
  return $Cmp_is_eq(cmp_new(_a_0, _b_0));
}

function $String_drop($0, $1) {
  for (;;) {
    {
      const _s_0 = $0;
      const _n_0 = $1;
      if (_s_0 === "") {
        return "";
      } else {
        const _h_0 = _s_0.slice(0, char_len(_s_0));
        const _t_0 = _s_0.slice(char_len(_s_0));
        if (_n_0 === 0) {
          return (_h_0 + _t_0);
        } else {
          const _p_0 = (_n_0 - 1);
          $0 = _t_0;
          $1 = _p_0;
          continue;
        }
      }
    }
  }
}

function $Equal_trans(_ab_0, _bc_0) {
  return _ab_0;
}

function $Equal_sym(_e_0) {
  return null;
}

function $Cmp_is_le(_c_0) {
  if (_c_0.$ === "GT") {
    return false;
  } else {
    return true;
  }
}

function $String_starts_with_go($0, $1, $2) {
  let $pc = 0;
  for (;;) switch ($pc) {
    case 0: {
      const _p_0 = $0;
      const _s_0 = $1;
      if (_p_0 === "") {
        return true;
      } else {
        const _y_0 = _p_0.slice(0, char_len(_p_0));
        const _yt_0 = _p_0.slice(char_len(_p_0));
        if (_s_0 === "") {
          return false;
        } else {
          const _x_0 = _s_0.slice(0, char_len(_s_0));
          const _xt_0 = _s_0.slice(char_len(_s_0));
          $0 = _yt_0;
          $1 = _xt_0;
          $2 = ($Char_is_eq(_x_0, _y_0));
          $pc = 1; continue;
        }
      }
    }
    case 1: {
      const _pt_0 = $0;
      const _t_0 = $1;
      const _same_0 = $2;
      if (!_same_0) {
        return false;
      } else {
        $0 = _pt_0;
        $1 = _t_0;
        $pc = 0; continue;
      }
    }
  }
}

function $Nat_read_trim($0) {
  for (;;) {
    {
      const _s_0 = $0;
      if (_s_0 !== "") {
        const _t_0 = _s_0.slice(0, char_len(_s_0));
        const _t_1 = char_code(_t_0);
        if (_t_1 == 48) {
          const _t_2 = _s_0.slice(char_len(_s_0));
          $0 = _t_2;
          continue;
        } else {
          const _10_0 = u32_to_word(_t_1)["head"];
          const _11_0 = u32_to_word(_t_1)["tail"];
          const _t_3 = _s_0.slice(char_len(_s_0));
          return (char_new(word_to_u32({$: "WCon", "head": _10_0, "tail": _11_0})) + _t_3);
        }
      } else {
        return _s_0;
      }
    }
  }
}

function $Nat_read_gate(_s_0, _ok_0) {
  if (_ok_0) {
    return $Nat_read_go(_s_0, 0);
  } else {
    return {$: "None"};
  }
}

function $Nat_read_fits(_s_0) {
  const _n_0 = [...str_out(_s_0)].length;
  const _x_0 = (_n_0 < 15);
  const _x_1 = ($Bool_and(($Nat_is_eq(_n_0, 15)), ($String_is_le(_s_0, "281474976710655"))));
  return (_x_0 || _x_1);
}

function $Bool_to_u32(_b_0) {
  if (!_b_0) {
    return 0;
  } else {
    return 1;
  }
}

function $Char_is_upper(_c_0) {
  const _x_0 = char_code(_c_0);
  const _x_1 = char_code(_c_0);
  return $Bool_and((_x_0 >= 65), (_x_1 <= 90));
}

function $Cmp_is_eq(_c_0) {
  if (_c_0.$ === "EQ") {
    return true;
  } else {
    return false;
  }
}

function $String_starts_with_go_if($0, $1, $2) {
  let $pc = 1;
  for (;;) switch ($pc) {
    case 0: {
      const _p_0 = $0;
      const _s_0 = $1;
      if (_p_0 === "") {
        return true;
      } else {
        const _y_0 = _p_0.slice(0, char_len(_p_0));
        const _yt_0 = _p_0.slice(char_len(_p_0));
        if (_s_0 === "") {
          return false;
        } else {
          const _x_0 = _s_0.slice(0, char_len(_s_0));
          const _xt_0 = _s_0.slice(char_len(_s_0));
          $0 = _yt_0;
          $1 = _xt_0;
          $2 = ($Char_is_eq(_x_0, _y_0));
          $pc = 1; continue;
        }
      }
    }
    case 1: {
      const _pt_0 = $0;
      const _t_0 = $1;
      const _same_0 = $2;
      if (!_same_0) {
        return false;
      } else {
        $0 = _pt_0;
        $1 = _t_0;
        $pc = 0; continue;
      }
    }
  }
}

function $Nat_read_go($0, $1, $2, $3) {
  let $pc = 0;
  for (;;) switch ($pc) {
    case 0: {
      const _s_0 = $0;
      const _acc_0 = $1;
      if (_s_0 === "") {
        return {$: "Some", "value": _acc_0};
      } else {
        const _t_0 = _s_0.slice(0, char_len(_s_0));
        const _t_1 = _s_0.slice(char_len(_s_0));
        const _x_0 = char_code(_t_0);
        const _d_0 = ((_x_0 - 48) >>> 0);
        $0 = _t_1;
        $1 = _acc_0;
        $2 = _d_0;
        $3 = (_d_0 < 10);
        $pc = 1; continue;
      }
    }
    case 1: {
      const _t_0 = $0;
      const _acc_0 = $1;
      const _d_0 = $2;
      const _ok_0 = $3;
      if (_ok_0) {
        const _x_0 = nat_chk(_acc_0 * 10);
        $0 = _t_0;
        $1 = nat_chk(_x_0 + _d_0);
        $pc = 0; continue;
      } else {
        return {$: "None"};
      }
    }
  }
}

function $String_is_le(_a_0, _b_0) {
  return $Cmp_is_le(($String_order(_a_0, _b_0)));
}

function $Nat_read_if($0, $1, $2, $3) {
  let $pc = 1;
  for (;;) switch ($pc) {
    case 0: {
      const _s_0 = $0;
      const _acc_0 = $1;
      if (_s_0 === "") {
        return {$: "Some", "value": _acc_0};
      } else {
        const _t_0 = _s_0.slice(0, char_len(_s_0));
        const _t_1 = _s_0.slice(char_len(_s_0));
        const _x_0 = char_code(_t_0);
        const _d_0 = ((_x_0 - 48) >>> 0);
        $0 = _t_1;
        $1 = _acc_0;
        $2 = _d_0;
        $3 = (_d_0 < 10);
        $pc = 1; continue;
      }
    }
    case 1: {
      const _t_0 = $0;
      const _acc_0 = $1;
      const _d_0 = $2;
      const _ok_0 = $3;
      if (_ok_0) {
        const _x_0 = nat_chk(_acc_0 * 10);
        $0 = _t_0;
        $1 = nat_chk(_x_0 + _d_0);
        $pc = 0; continue;
      } else {
        return {$: "None"};
      }
    }
  }
}

function $String_order(_a_0, _b_0) {
  return $String_order_go(_a_0, _b_0, {$: "EQ"});
}

function $String_order_go($0, $1, $2) {
  for (;;) {
    {
      const _a_0 = $0;
      const _b_0 = $1;
      const _c_0 = $2;
      if (_a_0 === "") {
        if (_b_0 !== "") {
          if (_c_0.$ === "EQ") {
            return {$: "LT"};
          } else {
            return _c_0;
          }
        } else {
          return _c_0;
        }
      } else {
        const _t_1 = _a_0.slice(0, char_len(_a_0));
        const _t_2 = _a_0.slice(char_len(_a_0));
        if (_b_0 === "") {
          if (_c_0.$ === "EQ") {
            return {$: "GT"};
          } else {
            return _c_0;
          }
        } else {
          const _t_3 = _b_0.slice(0, char_len(_b_0));
          const _t2_0 = _b_0.slice(char_len(_b_0));
          if (_c_0.$ === "EQ") {
            const _x_0 = char_code(_t_1);
            const _x_1 = char_code(_t_3);
            $0 = _t_2;
            $1 = _t2_0;
            $2 = cmp_new(_x_0, _x_1);
            continue;
          } else {
            return _c_0;
          }
        }
      }
    }
  }
}
export default {
  "packages/proven/bend/rate_limit:ok": run_lib((a0) => run_loop($packages_proven_bend_rate_limit_ok(a0)), 1),
  "packages/proven/bend/rate_limit:remaining": run_lib((a0) => run_loop($packages_proven_bend_rate_limit_remaining(a0)), 1),
  "packages/proven/bend/rate_limit:count": run_lib((a0) => run_loop($packages_proven_bend_rate_limit_count(a0)), 1),
  "packages/proven/bend/rate_limit:reset_at": run_lib((a0) => run_loop($packages_proven_bend_rate_limit_reset_at(a0)), 1),
  "packages/proven/bend/rate_limit:fresh": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_rate_limit_fresh(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "packages/proven/bend/rate_limit:admit.up": run_lib((a0) => run_loop($packages_proven_bend_rate_limit_admit_up(a0)), 1),
  "packages/proven/bend/rate_limit:admit": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_rate_limit_admit(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "packages/proven/bend/rate_limit:step.go": run_lib((a0,a1,a2,a3,a4,a5) => run_loop($packages_proven_bend_rate_limit_step_go(a0,nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4),nat_in(a5))), 6),
  "packages/proven/bend/rate_limit:step": run_lib((a0,a1,a2,a3) => run_loop($packages_proven_bend_rate_limit_step(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "packages/proven/bend/rate_limit:bump": run_lib((a0,a1) => run_loop($packages_proven_bend_rate_limit_bump(a0,nat_in(a1))), 2),
  "packages/proven/bend/freshness:term_end": run_lib((a0) => run_loop($packages_proven_bend_freshness_term_end(a0)), 1),
  "packages/proven/bend/freshness:day_ms": run_lib(() => run_loop($packages_proven_bend_freshness_day_ms()), 0),
  "packages/proven/bend/freshness:max_age_days": run_lib(() => run_loop($packages_proven_bend_freshness_max_age_days()), 0),
  "packages/proven/bend/freshness:last_seen_days": run_lib(() => run_loop($packages_proven_bend_freshness_last_seen_days()), 0),
  "packages/proven/bend/freshness:any_live": run_lib((a0,a1) => run_loop($packages_proven_bend_freshness_any_live(a0,nat_in(a1))), 2),
  "packages/proven/bend/freshness:all_past": run_lib((a0,a1) => run_loop($packages_proven_bend_freshness_all_past(a0,nat_in(a1))), 2),
  "packages/proven/bend/freshness:is_fresh": run_lib((a0,a1,a2,a3) => run_loop($packages_proven_bend_freshness_is_fresh(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "packages/proven/bend/freshness:is_stale": run_lib((a0,a1,a2,a3) => run_loop($packages_proven_bend_freshness_is_stale(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "packages/proven/bend/freshness:is_public": run_lib((a0,a1,a2,a3,a4) => run_loop($packages_proven_bend_freshness_is_public(nat_in(a0),a1,nat_in(a2),nat_in(a3),nat_in(a4))), 5),
  "packages/proven/bend/freshness:stale_last_seen": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_freshness_stale_last_seen(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "packages/proven/bend/apply_picker:needs_picker": run_lib((a0,a1) => run_loop($packages_proven_bend_apply_picker_needs_picker(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/apply_picker:qualify_single_title": run_lib((a0) => run_loop($packages_proven_bend_apply_picker_qualify_single_title(nat_in(a0))), 1),
  "packages/proven/bend/apply_picker:says_openings": run_lib((a0,a1) => run_loop($packages_proven_bend_apply_picker_says_openings(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/deactivation:policy": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_deactivation_policy(a0,nat_in(a1),nat_in(a2))), 3),
  "packages/proven/bend/funnel:or0": run_lib((a0) => run_loop($packages_proven_bend_funnel_or0(a0)), 1),
  "packages/proven/bend/funnel:zero": run_lib(() => run_loop($packages_proven_bend_funnel_zero()), 0),
  "packages/proven/bend/funnel:of": run_lib((a0) => run_loop($packages_proven_bend_funnel_of(a0)), 1),
  "packages/proven/bend/funnel:plus": run_lib((a0,a1) => run_loop($packages_proven_bend_funnel_plus(a0,a1)), 2),
  "packages/proven/bend/funnel:total": run_lib((a0) => run_loop($packages_proven_bend_funnel_total(a0)), 1),
  "packages/proven/bend/funnel:zero_match_tag": run_lib(() => str_out(run_loop($packages_proven_bend_funnel_zero_match_tag())), 0),
  "packages/proven/bend/funnel:is_zero_match": run_lib((a0) => run_loop($packages_proven_bend_funnel_is_zero_match(str_in(a0))), 1),
  "packages/proven/bend/pagination:pred_or": run_lib((a0,a1) => run_loop($packages_proven_bend_pagination_pred_or(a0,nat_in(a1))), 2),
  "packages/proven/bend/pagination:parse": run_lib((a0) => run_loop($packages_proven_bend_pagination_parse(a0)), 1),
  "packages/proven/bend/pagination:clamp": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_pagination_clamp(a0,nat_in(a1),nat_in(a2))), 3),
  "packages/proven/bend/pagination:page": run_lib((a0) => run_loop($packages_proven_bend_pagination_page(a0)), 1),
  "packages/proven/bend/pagination:limit": run_lib((a0) => run_loop($packages_proven_bend_pagination_limit(a0)), 1),
  "packages/proven/bend/pagination:offset": run_lib((a0,a1) => run_loop($packages_proven_bend_pagination_offset(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/job_slug:hex_char": run_lib((a0) => str_out(run_loop($packages_proven_bend_job_slug_hex_char(a0))), 1),
  "packages/proven/bend/job_slug:hex_str": run_lib((a0) => str_out(run_loop($packages_proven_bend_job_slug_hex_str(a0))), 1),
  "packages/proven/bend/job_slug:hex8_str": run_lib((a0) => str_out(run_loop($packages_proven_bend_job_slug_hex8_str(a0))), 1),
  "packages/proven/bend/job_slug:le_f": run_lib((a0) => run_loop($packages_proven_bend_job_slug_le_f(str_in(a0))), 1),
  "packages/proven/bend/job_slug:is_hex": run_lib((a0) => run_loop($packages_proven_bend_job_slug_is_hex(str_in(a0))), 1),
  "packages/proven/bend/job_slug:hex_digits.if": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_job_slug_hex_digits_if(a0,str_in(a1),a2)), 3),
  "packages/proven/bend/job_slug:hex_digits": run_lib((a0) => run_loop($packages_proven_bend_job_slug_hex_digits(str_in(a0))), 1),
  "packages/proven/bend/job_slug:strip_dashes.if": run_lib((a0,a1,a2) => str_out(run_loop($packages_proven_bend_job_slug_strip_dashes_if(a0,str_in(a1),str_in(a2)))), 3),
  "packages/proven/bend/job_slug:strip_dashes": run_lib((a0) => str_out(run_loop($packages_proven_bend_job_slug_strip_dashes(str_in(a0)))), 1),
  "packages/proven/bend/job_slug:suffix": run_lib((a0) => str_out(run_loop($packages_proven_bend_job_slug_suffix(str_in(a0)))), 1),
  "packages/proven/bend/job_slug:segment": run_lib((a0,a1) => str_out(run_loop($packages_proven_bend_job_slug_segment(str_in(a0),str_in(a1)))), 2),
  "packages/proven/bend/job_slug:parse.dash": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_job_slug_parse_dash(a0,a1,a2)), 3),
  "packages/proven/bend/job_slug:parse.tail": run_lib((a0) => run_loop($packages_proven_bend_job_slug_parse_tail(str_in(a0))), 1),
  "packages/proven/bend/job_slug:parse": run_lib((a0) => run_loop($packages_proven_bend_job_slug_parse(str_in(a0))), 1),
  "LAWS:rl_remaining": run_lib((a0,a1,a2,a3) => run_loop($LAWS_rl_remaining(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "LAWS:rl_bounded": run_lib((a0,a1,a2,a3) => run_loop($LAWS_rl_bounded(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "LAWS:rl_zero_limit": run_lib((a0,a1,a2) => run_loop($LAWS_rl_zero_limit(a0,nat_in(a1),nat_in(a2))), 3),
  "LAWS:rl_expired": run_lib((a0,a1,a2,a3,a4) => run_loop($LAWS_rl_expired(nat_in(a0),nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4))), 5),
  "LAWS:rl_open_window": run_lib((a0,a1,a2,a3,a4) => run_loop($LAWS_rl_open_window(nat_in(a0),nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4))), 5),
  "LAWS:rl_admit_count": run_lib((a0,a1,a2) => run_loop($LAWS_rl_admit_count(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "LAWS:rl_admit_keeps_window": run_lib((a0,a1,a2) => run_loop($LAWS_rl_admit_keeps_window(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "LAWS:rl_fresh_window": run_lib((a0,a1,a2) => run_loop($LAWS_rl_fresh_window(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "LAWS:fr_stale_iff_not_fresh": run_lib((a0,a1,a2,a3) => run_loop($LAWS_fr_stale_iff_not_fresh(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "LAWS:fr_stale_monotone": run_lib((a0,a1,a2,a3,a4,a5) => run_loop($LAWS_fr_stale_monotone(a0,nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4),a5)), 6),
  "LAWS:fr_last_seen_monotone": run_lib((a0,a1,a2,a3,a4) => run_loop($LAWS_fr_last_seen_monotone(nat_in(a0),nat_in(a1),nat_in(a2),nat_in(a3),a4)), 5),
  "LAWS:fr_term_live_at_end": run_lib((a0,a1,a2,a3) => run_loop($LAWS_fr_term_live_at_end(nat_in(a0),a1,nat_in(a2),nat_in(a3))), 4),
  "LAWS:fr_age_live_at_limit": run_lib((a0,a1) => run_loop($LAWS_fr_age_live_at_limit(nat_in(a0),nat_in(a1))), 2),
  "LAWS:fr_public_needs_role": run_lib((a0,a1,a2,a3) => run_loop($LAWS_fr_public_needs_role(a0,nat_in(a1),nat_in(a2),nat_in(a3))), 4),
  "LAWS:fr_public_is_fresh": run_lib((a0,a1,a2,a3,a4) => run_loop($LAWS_fr_public_is_fresh(nat_in(a0),a1,nat_in(a2),nat_in(a3),nat_in(a4))), 5),
  "LAWS:fr_summer_end": run_lib(() => run_loop($LAWS_fr_summer_end()), 0),
  "LAWS:fr_fall_end": run_lib(() => run_loop($LAWS_fr_fall_end()), 0),
  "LAWS:fr_winter_end": run_lib(() => run_loop($LAWS_fr_winter_end()), 0),
  "LAWS:fr_other_end": run_lib(() => run_loop($LAWS_fr_other_end()), 0),
  "LAWS:ap_empty": run_lib((a0) => run_loop($LAWS_ap_empty(nat_in(a0))), 1),
  "LAWS:ap_many_postings": run_lib((a0,a1) => run_loop($LAWS_ap_many_postings(nat_in(a0),nat_in(a1))), 2),
  "LAWS:ap_one_posting": run_lib((a0) => run_loop($LAWS_ap_one_posting(nat_in(a0))), 1),
  "LAWS:ap_title_matches_picker": run_lib((a0) => run_loop($LAWS_ap_title_matches_picker(nat_in(a0))), 1),
  "LAWS:ap_distinct_says_locations": run_lib((a0) => run_loop($LAWS_ap_distinct_says_locations(nat_in(a0))), 1),
  "LAWS:ap_repeats_say_openings": run_lib((a0,a1) => run_loop($LAWS_ap_repeats_say_openings(nat_in(a0),nat_in(a1))), 2),
  "LAWS:dp_failed_fetch_keeps": run_lib((a0,a1) => run_loop($LAWS_dp_failed_fetch_keeps(nat_in(a0),nat_in(a1))), 2),
  "LAWS:dp_matches_retire_missing": run_lib((a0,a1) => run_loop($LAWS_dp_matches_retire_missing(nat_in(a0),nat_in(a1))), 2),
  "LAWS:dp_empty_board_retires_all": run_lib(() => run_loop($LAWS_dp_empty_board_retires_all()), 0),
  "LAWS:dp_zero_match_keeps": run_lib((a0) => run_loop($LAWS_dp_zero_match_keeps(nat_in(a0))), 1),
  "LAWS:fn_total_empty": run_lib(() => run_loop($LAWS_fn_total_empty()), 0),
  "LAWS:fn_total_append": run_lib((a0,a1) => run_loop($LAWS_fn_total_append(a0,a1)), 2),
  "LAWS:fn_zero_match_tagged": run_lib((a0) => run_loop($LAWS_fn_zero_match_tagged(str_in(a0))), 1),
  "LAWS:fn_funnel_not_zero_match": run_lib((a0) => run_loop($LAWS_fn_funnel_not_zero_match(str_in(a0))), 1),
  "LAWS:pg_page_min": run_lib((a0) => run_loop($LAWS_pg_page_min(a0)), 1),
  "LAWS:pg_page_max": run_lib((a0) => run_loop($LAWS_pg_page_max(a0)), 1),
  "LAWS:pg_limit_min": run_lib((a0) => run_loop($LAWS_pg_limit_min(a0)), 1),
  "LAWS:pg_limit_max": run_lib((a0) => run_loop($LAWS_pg_limit_max(a0)), 1),
  "LAWS:pg_defaults": run_lib(() => run_loop($LAWS_pg_defaults()), 0),
  "LAWS:pg_first_page_offset": run_lib((a0) => run_loop($LAWS_pg_first_page_offset(nat_in(a0))), 1),
  "LAWS:js_roundtrip": run_lib((a0,a1) => run_loop($LAWS_js_roundtrip(str_in(a0),a1)), 2),
  "packages/proven/bend/lemmas:TrueDisc": run_lib((a0) => run_loop($packages_proven_bend_lemmas_TrueDisc(a0)), 1),
  "packages/proven/bend/lemmas:ft_empty": run_lib((a0) => run_loop($packages_proven_bend_lemmas_ft_empty(a0)), 1),
  "packages/proven/bend/lemmas:empty_elim": run_lib((a0) => run_loop($packages_proven_bend_lemmas_empty_elim(a0)), 1),
  "packages/proven/bend/lemmas:ft": run_lib((a0) => run_loop($packages_proven_bend_lemmas_ft(a0)), 1),
  "packages/proven/bend/lemmas:not_or": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_not_or(a0,a1)), 2),
  "packages/proven/bend/lemmas:not_not": run_lib((a0) => run_loop($packages_proven_bend_lemmas_not_not(a0)), 1),
  "packages/proven/bend/lemmas:le_refl": run_lib((a0) => run_loop($packages_proven_bend_lemmas_le_refl(nat_in(a0))), 1),
  "packages/proven/bend/lemmas:le_add": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_le_add(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/lemmas:lt_add_succ": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_lt_add_succ(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/lemmas:lt_not_le": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_lt_not_le(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/lemmas:add_zero": run_lib((a0) => run_loop($packages_proven_bend_lemmas_add_zero(nat_in(a0))), 1),
  "packages/proven/bend/lemmas:add_succ": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_add_succ(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/lemmas:add_assoc": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_lemmas_add_assoc(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "packages/proven/bend/lemmas:sub_zero": run_lib((a0) => run_loop($packages_proven_bend_lemmas_sub_zero(nat_in(a0))), 1),
  "packages/proven/bend/lemmas:add_sub": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_add_sub(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/lemmas:and_l": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_lemmas_and_l(a0,a1,a2)), 3),
  "packages/proven/bend/lemmas:and_r": run_lib((a0,a1,a2) => run_loop($packages_proven_bend_lemmas_and_r(a0,a1,a2)), 3),
  "packages/proven/bend/lemmas:and_intro": run_lib((a0,a1,a2,a3) => run_loop($packages_proven_bend_lemmas_and_intro(a0,a1,a2,a3)), 4),
  "packages/proven/bend/lemmas:lt_mono": run_lib((a0,a1,a2,a3) => run_loop($packages_proven_bend_lemmas_lt_mono(nat_in(a0),nat_in(a1),nat_in(a2),a3)), 4),
  "packages/proven/bend/lemmas:lt_irrefl": run_lib((a0) => run_loop($packages_proven_bend_lemmas_lt_irrefl(nat_in(a0))), 1),
  "packages/proven/bend/lemmas:lt_succ_add": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_lt_succ_add(nat_in(a0),nat_in(a1))), 2),
  "packages/proven/bend/lemmas:le_one_succ": run_lib((a0) => run_loop($packages_proven_bend_lemmas_le_one_succ(nat_in(a0))), 1),
  "packages/proven/bend/lemmas:min_le_l": run_lib((a0,a1) => run_loop($packages_proven_bend_lemmas_min_le_l(nat_in(a0),nat_in(a1))), 2),
  "rl.up_ok": run_lib((a0) => run_loop($rl_up_ok(a0)), 1),
  "rl.up_rem": run_lib((a0) => run_loop($rl_up_rem(a0)), 1),
  "rl.up_count": run_lib((a0) => run_loop($rl_up_count(a0)), 1),
  "rl.up_reset": run_lib((a0) => run_loop($rl_up_reset(a0)), 1),
  "rl.bump_succ": run_lib((a0,a1) => run_loop($rl_bump_succ(a0,nat_in(a1))), 2),
  "rl.admit_rem": run_lib((a0,a1,a2) => run_loop($rl_admit_rem(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "rl.admit_bounded": run_lib((a0,a1,a2) => run_loop($rl_admit_bounded(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "rl.admit_zero": run_lib((a0,a1) => run_loop($rl_admit_zero(nat_in(a0),nat_in(a1))), 2),
  "rl.admit_count": run_lib((a0,a1,a2) => run_loop($rl_admit_count(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "rl.admit_reset": run_lib((a0,a1,a2) => run_loop($rl_admit_reset(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "rl.fresh_rem": run_lib((a0,a1,a2) => run_loop($rl_fresh_rem(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "rl.fresh_bounded": run_lib((a0,a1,a2) => run_loop($rl_fresh_bounded(nat_in(a0),nat_in(a1),nat_in(a2))), 3),
  "rl.go_rem": run_lib((a0,a1,a2,a3,a4,a5) => run_loop($rl_go_rem(a0,nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4),nat_in(a5))), 6),
  "rl.go_bounded": run_lib((a0,a1,a2,a3,a4,a5) => run_loop($rl_go_bounded(a0,nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4),nat_in(a5))), 6),
  "rl.go_zero": run_lib((a0,a1,a2,a3,a4) => run_loop($rl_go_zero(a0,nat_in(a1),nat_in(a2),nat_in(a3),nat_in(a4))), 5),
  "fr.past_live": run_lib((a0,a1) => run_loop($fr_past_live(a0,nat_in(a1))), 2),
  "fr.all_past_mono": run_lib((a0,a1,a2,a3) => run_loop($fr_all_past_mono(a0,nat_in(a1),nat_in(a2),a3)), 4),
  "fn.zero_plus": run_lib((a0) => run_loop($fn_zero_plus(a0)), 1),
  "fn.plus_assoc": run_lib((a0,a1,a2) => run_loop($fn_plus_assoc(a0,a1,a2)), 3),
  "js.len_append": run_lib((a0,a1) => run_loop($js_len_append(str_in(a0),str_in(a1))), 2),
  "js.drop_append": run_lib((a0,a1) => run_loop($js_drop_append(str_in(a0),str_in(a1))), 2),
  "js.step": run_lib((a0,a1) => run_loop($js_step(a0,str_in(a1))), 2),
  "js.digits_hex": run_lib((a0) => run_loop($js_digits_hex(a0)), 1),
};

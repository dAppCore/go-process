/**
 * @license
 * Copyright 2019 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const V = globalThis, ie = V.ShadowRoot && (V.ShadyCSS === void 0 || V.ShadyCSS.nativeShadow) && "adoptedStyleSheets" in Document.prototype && "replace" in CSSStyleSheet.prototype, re = Symbol(), ae = /* @__PURE__ */ new WeakMap();
let ye = class {
  constructor(e, t, i) {
    if (this._$cssResult$ = !0, i !== re) throw Error("CSSResult is not constructable. Use `unsafeCSS` or `css` instead.");
    this.cssText = e, this.t = t;
  }
  get styleSheet() {
    let e = this.o;
    const t = this.t;
    if (ie && e === void 0) {
      const i = t !== void 0 && t.length === 1;
      i && (e = ae.get(t)), e === void 0 && ((this.o = e = new CSSStyleSheet()).replaceSync(this.cssText), i && ae.set(t, e));
    }
    return e;
  }
  toString() {
    return this.cssText;
  }
};
const ke = (s) => new ye(typeof s == "string" ? s : s + "", void 0, re), B = (s, ...e) => {
  const t = s.length === 1 ? s[0] : e.reduce((i, r, n) => i + ((o) => {
    if (o._$cssResult$ === !0) return o.cssText;
    if (typeof o == "number") return o;
    throw Error("Value passed to 'css' function must be a 'css' function result: " + o + ". Use 'unsafeCSS' to pass non-literal values, but take care to ensure page security.");
  })(r) + s[n + 1], s[0]);
  return new ye(t, s, re);
}, Se = (s, e) => {
  if (ie) s.adoptedStyleSheets = e.map((t) => t instanceof CSSStyleSheet ? t : t.styleSheet);
  else for (const t of e) {
    const i = document.createElement("style"), r = V.litNonce;
    r !== void 0 && i.setAttribute("nonce", r), i.textContent = t.cssText, s.appendChild(i);
  }
}, le = ie ? (s) => s : (s) => s instanceof CSSStyleSheet ? ((e) => {
  let t = "";
  for (const i of e.cssRules) t += i.cssText;
  return ke(t);
})(s) : s;
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const { is: Pe, defineProperty: Ce, getOwnPropertyDescriptor: Ee, getOwnPropertyNames: Ue, getOwnPropertySymbols: Oe, getPrototypeOf: ze } = Object, A = globalThis, ce = A.trustedTypes, Te = ce ? ce.emptyScript : "", Y = A.reactiveElementPolyfillSupport, j = (s, e) => s, J = { toAttribute(s, e) {
  switch (e) {
    case Boolean:
      s = s ? Te : null;
      break;
    case Object:
    case Array:
      s = s == null ? s : JSON.stringify(s);
  }
  return s;
}, fromAttribute(s, e) {
  let t = s;
  switch (e) {
    case Boolean:
      t = s !== null;
      break;
    case Number:
      t = s === null ? null : Number(s);
      break;
    case Object:
    case Array:
      try {
        t = JSON.parse(s);
      } catch {
        t = null;
      }
  }
  return t;
} }, oe = (s, e) => !Pe(s, e), de = { attribute: !0, type: String, converter: J, reflect: !1, useDefault: !1, hasChanged: oe };
Symbol.metadata ?? (Symbol.metadata = Symbol("metadata")), A.litPropertyMetadata ?? (A.litPropertyMetadata = /* @__PURE__ */ new WeakMap());
let T = class extends HTMLElement {
  static addInitializer(e) {
    this._$Ei(), (this.l ?? (this.l = [])).push(e);
  }
  static get observedAttributes() {
    return this.finalize(), this._$Eh && [...this._$Eh.keys()];
  }
  static createProperty(e, t = de) {
    if (t.state && (t.attribute = !1), this._$Ei(), this.prototype.hasOwnProperty(e) && ((t = Object.create(t)).wrapped = !0), this.elementProperties.set(e, t), !t.noAccessor) {
      const i = Symbol(), r = this.getPropertyDescriptor(e, i, t);
      r !== void 0 && Ce(this.prototype, e, r);
    }
  }
  static getPropertyDescriptor(e, t, i) {
    const { get: r, set: n } = Ee(this.prototype, e) ?? { get() {
      return this[t];
    }, set(o) {
      this[t] = o;
    } };
    return { get: r, set(o) {
      const l = r == null ? void 0 : r.call(this);
      n == null || n.call(this, o), this.requestUpdate(e, l, i);
    }, configurable: !0, enumerable: !0 };
  }
  static getPropertyOptions(e) {
    return this.elementProperties.get(e) ?? de;
  }
  static _$Ei() {
    if (this.hasOwnProperty(j("elementProperties"))) return;
    const e = ze(this);
    e.finalize(), e.l !== void 0 && (this.l = [...e.l]), this.elementProperties = new Map(e.elementProperties);
  }
  static finalize() {
    if (this.hasOwnProperty(j("finalized"))) return;
    if (this.finalized = !0, this._$Ei(), this.hasOwnProperty(j("properties"))) {
      const t = this.properties, i = [...Ue(t), ...Oe(t)];
      for (const r of i) this.createProperty(r, t[r]);
    }
    const e = this[Symbol.metadata];
    if (e !== null) {
      const t = litPropertyMetadata.get(e);
      if (t !== void 0) for (const [i, r] of t) this.elementProperties.set(i, r);
    }
    this._$Eh = /* @__PURE__ */ new Map();
    for (const [t, i] of this.elementProperties) {
      const r = this._$Eu(t, i);
      r !== void 0 && this._$Eh.set(r, t);
    }
    this.elementStyles = this.finalizeStyles(this.styles);
  }
  static finalizeStyles(e) {
    const t = [];
    if (Array.isArray(e)) {
      const i = new Set(e.flat(1 / 0).reverse());
      for (const r of i) t.unshift(le(r));
    } else e !== void 0 && t.push(le(e));
    return t;
  }
  static _$Eu(e, t) {
    const i = t.attribute;
    return i === !1 ? void 0 : typeof i == "string" ? i : typeof e == "string" ? e.toLowerCase() : void 0;
  }
  constructor() {
    super(), this._$Ep = void 0, this.isUpdatePending = !1, this.hasUpdated = !1, this._$Em = null, this._$Ev();
  }
  _$Ev() {
    var e;
    this._$ES = new Promise((t) => this.enableUpdating = t), this._$AL = /* @__PURE__ */ new Map(), this._$E_(), this.requestUpdate(), (e = this.constructor.l) == null || e.forEach((t) => t(this));
  }
  addController(e) {
    var t;
    (this._$EO ?? (this._$EO = /* @__PURE__ */ new Set())).add(e), this.renderRoot !== void 0 && this.isConnected && ((t = e.hostConnected) == null || t.call(e));
  }
  removeController(e) {
    var t;
    (t = this._$EO) == null || t.delete(e);
  }
  _$E_() {
    const e = /* @__PURE__ */ new Map(), t = this.constructor.elementProperties;
    for (const i of t.keys()) this.hasOwnProperty(i) && (e.set(i, this[i]), delete this[i]);
    e.size > 0 && (this._$Ep = e);
  }
  createRenderRoot() {
    const e = this.shadowRoot ?? this.attachShadow(this.constructor.shadowRootOptions);
    return Se(e, this.constructor.elementStyles), e;
  }
  connectedCallback() {
    var e;
    this.renderRoot ?? (this.renderRoot = this.createRenderRoot()), this.enableUpdating(!0), (e = this._$EO) == null || e.forEach((t) => {
      var i;
      return (i = t.hostConnected) == null ? void 0 : i.call(t);
    });
  }
  enableUpdating(e) {
  }
  disconnectedCallback() {
    var e;
    (e = this._$EO) == null || e.forEach((t) => {
      var i;
      return (i = t.hostDisconnected) == null ? void 0 : i.call(t);
    });
  }
  attributeChangedCallback(e, t, i) {
    this._$AK(e, i);
  }
  _$ET(e, t) {
    var n;
    const i = this.constructor.elementProperties.get(e), r = this.constructor._$Eu(e, i);
    if (r !== void 0 && i.reflect === !0) {
      const o = (((n = i.converter) == null ? void 0 : n.toAttribute) !== void 0 ? i.converter : J).toAttribute(t, i.type);
      this._$Em = e, o == null ? this.removeAttribute(r) : this.setAttribute(r, o), this._$Em = null;
    }
  }
  _$AK(e, t) {
    var n, o;
    const i = this.constructor, r = i._$Eh.get(e);
    if (r !== void 0 && this._$Em !== r) {
      const l = i.getPropertyOptions(r), a = typeof l.converter == "function" ? { fromAttribute: l.converter } : ((n = l.converter) == null ? void 0 : n.fromAttribute) !== void 0 ? l.converter : J;
      this._$Em = r;
      const p = a.fromAttribute(t, l.type);
      this[r] = p ?? ((o = this._$Ej) == null ? void 0 : o.get(r)) ?? p, this._$Em = null;
    }
  }
  requestUpdate(e, t, i, r = !1, n) {
    var o;
    if (e !== void 0) {
      const l = this.constructor;
      if (r === !1 && (n = this[e]), i ?? (i = l.getPropertyOptions(e)), !((i.hasChanged ?? oe)(n, t) || i.useDefault && i.reflect && n === ((o = this._$Ej) == null ? void 0 : o.get(e)) && !this.hasAttribute(l._$Eu(e, i)))) return;
      this.C(e, t, i);
    }
    this.isUpdatePending === !1 && (this._$ES = this._$EP());
  }
  C(e, t, { useDefault: i, reflect: r, wrapped: n }, o) {
    i && !(this._$Ej ?? (this._$Ej = /* @__PURE__ */ new Map())).has(e) && (this._$Ej.set(e, o ?? t ?? this[e]), n !== !0 || o !== void 0) || (this._$AL.has(e) || (this.hasUpdated || i || (t = void 0), this._$AL.set(e, t)), r === !0 && this._$Em !== e && (this._$Eq ?? (this._$Eq = /* @__PURE__ */ new Set())).add(e));
  }
  async _$EP() {
    this.isUpdatePending = !0;
    try {
      await this._$ES;
    } catch (t) {
      Promise.reject(t);
    }
    const e = this.scheduleUpdate();
    return e != null && await e, !this.isUpdatePending;
  }
  scheduleUpdate() {
    return this.performUpdate();
  }
  performUpdate() {
    var i;
    if (!this.isUpdatePending) return;
    if (!this.hasUpdated) {
      if (this.renderRoot ?? (this.renderRoot = this.createRenderRoot()), this._$Ep) {
        for (const [n, o] of this._$Ep) this[n] = o;
        this._$Ep = void 0;
      }
      const r = this.constructor.elementProperties;
      if (r.size > 0) for (const [n, o] of r) {
        const { wrapped: l } = o, a = this[n];
        l !== !0 || this._$AL.has(n) || a === void 0 || this.C(n, void 0, o, a);
      }
    }
    let e = !1;
    const t = this._$AL;
    try {
      e = this.shouldUpdate(t), e ? (this.willUpdate(t), (i = this._$EO) == null || i.forEach((r) => {
        var n;
        return (n = r.hostUpdate) == null ? void 0 : n.call(r);
      }), this.update(t)) : this._$EM();
    } catch (r) {
      throw e = !1, this._$EM(), r;
    }
    e && this._$AE(t);
  }
  willUpdate(e) {
  }
  _$AE(e) {
    var t;
    (t = this._$EO) == null || t.forEach((i) => {
      var r;
      return (r = i.hostUpdated) == null ? void 0 : r.call(i);
    }), this.hasUpdated || (this.hasUpdated = !0, this.firstUpdated(e)), this.updated(e);
  }
  _$EM() {
    this._$AL = /* @__PURE__ */ new Map(), this.isUpdatePending = !1;
  }
  get updateComplete() {
    return this.getUpdateComplete();
  }
  getUpdateComplete() {
    return this._$ES;
  }
  shouldUpdate(e) {
    return !0;
  }
  update(e) {
    this._$Eq && (this._$Eq = this._$Eq.forEach((t) => this._$ET(t, this[t]))), this._$EM();
  }
  updated(e) {
  }
  firstUpdated(e) {
  }
};
T.elementStyles = [], T.shadowRootOptions = { mode: "open" }, T[j("elementProperties")] = /* @__PURE__ */ new Map(), T[j("finalized")] = /* @__PURE__ */ new Map(), Y == null || Y({ ReactiveElement: T }), (A.reactiveElementVersions ?? (A.reactiveElementVersions = [])).push("2.1.2");
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const N = globalThis, he = (s) => s, Z = N.trustedTypes, pe = Z ? Z.createPolicy("lit-html", { createHTML: (s) => s }) : void 0, ve = "$lit$", x = `lit$${Math.random().toFixed(9).slice(2)}$`, _e = "?" + x, De = `<${_e}>`, U = document, I = () => U.createComment(""), L = (s) => s === null || typeof s != "object" && typeof s != "function", ne = Array.isArray, Me = (s) => ne(s) || typeof (s == null ? void 0 : s[Symbol.iterator]) == "function", ee = `[ 	
\f\r]`, R = /<(?:(!--|\/[^a-zA-Z])|(\/?[a-zA-Z][^>\s]*)|(\/?$))/g, ue = /-->/g, me = />/g, P = RegExp(`>|${ee}(?:([^\\s"'>=/]+)(${ee}*=${ee}*(?:[^ 	
\f\r"'\`<>=]|("|')|))|$)`, "g"), fe = /'/g, ge = /"/g, we = /^(?:script|style|textarea|title)$/i, He = (s) => (e, ...t) => ({ _$litType$: s, strings: e, values: t }), c = He(1), D = Symbol.for("lit-noChange"), d = Symbol.for("lit-nothing"), be = /* @__PURE__ */ new WeakMap(), C = U.createTreeWalker(U, 129);
function xe(s, e) {
  if (!ne(s) || !s.hasOwnProperty("raw")) throw Error("invalid template strings array");
  return pe !== void 0 ? pe.createHTML(e) : e;
}
const Re = (s, e) => {
  const t = s.length - 1, i = [];
  let r, n = e === 2 ? "<svg>" : e === 3 ? "<math>" : "", o = R;
  for (let l = 0; l < t; l++) {
    const a = s[l];
    let p, m, h = -1, $ = 0;
    for (; $ < a.length && (o.lastIndex = $, m = o.exec(a), m !== null); ) $ = o.lastIndex, o === R ? m[1] === "!--" ? o = ue : m[1] !== void 0 ? o = me : m[2] !== void 0 ? (we.test(m[2]) && (r = RegExp("</" + m[2], "g")), o = P) : m[3] !== void 0 && (o = P) : o === P ? m[0] === ">" ? (o = r ?? R, h = -1) : m[1] === void 0 ? h = -2 : (h = o.lastIndex - m[2].length, p = m[1], o = m[3] === void 0 ? P : m[3] === '"' ? ge : fe) : o === ge || o === fe ? o = P : o === ue || o === me ? o = R : (o = P, r = void 0);
    const w = o === P && s[l + 1].startsWith("/>") ? " " : "";
    n += o === R ? a + De : h >= 0 ? (i.push(p), a.slice(0, h) + ve + a.slice(h) + x + w) : a + x + (h === -2 ? l : w);
  }
  return [xe(s, n + (s[t] || "<?>") + (e === 2 ? "</svg>" : e === 3 ? "</math>" : "")), i];
};
class q {
  constructor({ strings: e, _$litType$: t }, i) {
    let r;
    this.parts = [];
    let n = 0, o = 0;
    const l = e.length - 1, a = this.parts, [p, m] = Re(e, t);
    if (this.el = q.createElement(p, i), C.currentNode = this.el.content, t === 2 || t === 3) {
      const h = this.el.content.firstChild;
      h.replaceWith(...h.childNodes);
    }
    for (; (r = C.nextNode()) !== null && a.length < l; ) {
      if (r.nodeType === 1) {
        if (r.hasAttributes()) for (const h of r.getAttributeNames()) if (h.endsWith(ve)) {
          const $ = m[o++], w = r.getAttribute(h).split(x), K = /([.?@])?(.*)/.exec($);
          a.push({ type: 1, index: n, name: K[2], strings: w, ctor: K[1] === "." ? Ne : K[1] === "?" ? Ie : K[1] === "@" ? Le : Q }), r.removeAttribute(h);
        } else h.startsWith(x) && (a.push({ type: 6, index: n }), r.removeAttribute(h));
        if (we.test(r.tagName)) {
          const h = r.textContent.split(x), $ = h.length - 1;
          if ($ > 0) {
            r.textContent = Z ? Z.emptyScript : "";
            for (let w = 0; w < $; w++) r.append(h[w], I()), C.nextNode(), a.push({ type: 2, index: ++n });
            r.append(h[$], I());
          }
        }
      } else if (r.nodeType === 8) if (r.data === _e) a.push({ type: 2, index: n });
      else {
        let h = -1;
        for (; (h = r.data.indexOf(x, h + 1)) !== -1; ) a.push({ type: 7, index: n }), h += x.length - 1;
      }
      n++;
    }
  }
  static createElement(e, t) {
    const i = U.createElement("template");
    return i.innerHTML = e, i;
  }
}
function M(s, e, t = s, i) {
  var o, l;
  if (e === D) return e;
  let r = i !== void 0 ? (o = t._$Co) == null ? void 0 : o[i] : t._$Cl;
  const n = L(e) ? void 0 : e._$litDirective$;
  return (r == null ? void 0 : r.constructor) !== n && ((l = r == null ? void 0 : r._$AO) == null || l.call(r, !1), n === void 0 ? r = void 0 : (r = new n(s), r._$AT(s, t, i)), i !== void 0 ? (t._$Co ?? (t._$Co = []))[i] = r : t._$Cl = r), r !== void 0 && (e = M(s, r._$AS(s, e.values), r, i)), e;
}
class je {
  constructor(e, t) {
    this._$AV = [], this._$AN = void 0, this._$AD = e, this._$AM = t;
  }
  get parentNode() {
    return this._$AM.parentNode;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  u(e) {
    const { el: { content: t }, parts: i } = this._$AD, r = ((e == null ? void 0 : e.creationScope) ?? U).importNode(t, !0);
    C.currentNode = r;
    let n = C.nextNode(), o = 0, l = 0, a = i[0];
    for (; a !== void 0; ) {
      if (o === a.index) {
        let p;
        a.type === 2 ? p = new F(n, n.nextSibling, this, e) : a.type === 1 ? p = new a.ctor(n, a.name, a.strings, this, e) : a.type === 6 && (p = new qe(n, this, e)), this._$AV.push(p), a = i[++l];
      }
      o !== (a == null ? void 0 : a.index) && (n = C.nextNode(), o++);
    }
    return C.currentNode = U, r;
  }
  p(e) {
    let t = 0;
    for (const i of this._$AV) i !== void 0 && (i.strings !== void 0 ? (i._$AI(e, i, t), t += i.strings.length - 2) : i._$AI(e[t])), t++;
  }
}
class F {
  get _$AU() {
    var e;
    return ((e = this._$AM) == null ? void 0 : e._$AU) ?? this._$Cv;
  }
  constructor(e, t, i, r) {
    this.type = 2, this._$AH = d, this._$AN = void 0, this._$AA = e, this._$AB = t, this._$AM = i, this.options = r, this._$Cv = (r == null ? void 0 : r.isConnected) ?? !0;
  }
  get parentNode() {
    let e = this._$AA.parentNode;
    const t = this._$AM;
    return t !== void 0 && (e == null ? void 0 : e.nodeType) === 11 && (e = t.parentNode), e;
  }
  get startNode() {
    return this._$AA;
  }
  get endNode() {
    return this._$AB;
  }
  _$AI(e, t = this) {
    e = M(this, e, t), L(e) ? e === d || e == null || e === "" ? (this._$AH !== d && this._$AR(), this._$AH = d) : e !== this._$AH && e !== D && this._(e) : e._$litType$ !== void 0 ? this.$(e) : e.nodeType !== void 0 ? this.T(e) : Me(e) ? this.k(e) : this._(e);
  }
  O(e) {
    return this._$AA.parentNode.insertBefore(e, this._$AB);
  }
  T(e) {
    this._$AH !== e && (this._$AR(), this._$AH = this.O(e));
  }
  _(e) {
    this._$AH !== d && L(this._$AH) ? this._$AA.nextSibling.data = e : this.T(U.createTextNode(e)), this._$AH = e;
  }
  $(e) {
    var n;
    const { values: t, _$litType$: i } = e, r = typeof i == "number" ? this._$AC(e) : (i.el === void 0 && (i.el = q.createElement(xe(i.h, i.h[0]), this.options)), i);
    if (((n = this._$AH) == null ? void 0 : n._$AD) === r) this._$AH.p(t);
    else {
      const o = new je(r, this), l = o.u(this.options);
      o.p(t), this.T(l), this._$AH = o;
    }
  }
  _$AC(e) {
    let t = be.get(e.strings);
    return t === void 0 && be.set(e.strings, t = new q(e)), t;
  }
  k(e) {
    ne(this._$AH) || (this._$AH = [], this._$AR());
    const t = this._$AH;
    let i, r = 0;
    for (const n of e) r === t.length ? t.push(i = new F(this.O(I()), this.O(I()), this, this.options)) : i = t[r], i._$AI(n), r++;
    r < t.length && (this._$AR(i && i._$AB.nextSibling, r), t.length = r);
  }
  _$AR(e = this._$AA.nextSibling, t) {
    var i;
    for ((i = this._$AP) == null ? void 0 : i.call(this, !1, !0, t); e !== this._$AB; ) {
      const r = he(e).nextSibling;
      he(e).remove(), e = r;
    }
  }
  setConnected(e) {
    var t;
    this._$AM === void 0 && (this._$Cv = e, (t = this._$AP) == null || t.call(this, e));
  }
}
class Q {
  get tagName() {
    return this.element.tagName;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  constructor(e, t, i, r, n) {
    this.type = 1, this._$AH = d, this._$AN = void 0, this.element = e, this.name = t, this._$AM = r, this.options = n, i.length > 2 || i[0] !== "" || i[1] !== "" ? (this._$AH = Array(i.length - 1).fill(new String()), this.strings = i) : this._$AH = d;
  }
  _$AI(e, t = this, i, r) {
    const n = this.strings;
    let o = !1;
    if (n === void 0) e = M(this, e, t, 0), o = !L(e) || e !== this._$AH && e !== D, o && (this._$AH = e);
    else {
      const l = e;
      let a, p;
      for (e = n[0], a = 0; a < n.length - 1; a++) p = M(this, l[i + a], t, a), p === D && (p = this._$AH[a]), o || (o = !L(p) || p !== this._$AH[a]), p === d ? e = d : e !== d && (e += (p ?? "") + n[a + 1]), this._$AH[a] = p;
    }
    o && !r && this.j(e);
  }
  j(e) {
    e === d ? this.element.removeAttribute(this.name) : this.element.setAttribute(this.name, e ?? "");
  }
}
class Ne extends Q {
  constructor() {
    super(...arguments), this.type = 3;
  }
  j(e) {
    this.element[this.name] = e === d ? void 0 : e;
  }
}
class Ie extends Q {
  constructor() {
    super(...arguments), this.type = 4;
  }
  j(e) {
    this.element.toggleAttribute(this.name, !!e && e !== d);
  }
}
class Le extends Q {
  constructor(e, t, i, r, n) {
    super(e, t, i, r, n), this.type = 5;
  }
  _$AI(e, t = this) {
    if ((e = M(this, e, t, 0) ?? d) === D) return;
    const i = this._$AH, r = e === d && i !== d || e.capture !== i.capture || e.once !== i.once || e.passive !== i.passive, n = e !== d && (i === d || r);
    r && this.element.removeEventListener(this.name, this, i), n && this.element.addEventListener(this.name, this, e), this._$AH = e;
  }
  handleEvent(e) {
    var t;
    typeof this._$AH == "function" ? this._$AH.call(((t = this.options) == null ? void 0 : t.host) ?? this.element, e) : this._$AH.handleEvent(e);
  }
}
class qe {
  constructor(e, t, i) {
    this.element = e, this.type = 6, this._$AN = void 0, this._$AM = t, this.options = i;
  }
  get _$AU() {
    return this._$AM._$AU;
  }
  _$AI(e) {
    M(this, e);
  }
}
const te = N.litHtmlPolyfillSupport;
te == null || te(q, F), (N.litHtmlVersions ?? (N.litHtmlVersions = [])).push("3.3.2");
const Be = (s, e, t) => {
  const i = (t == null ? void 0 : t.renderBefore) ?? e;
  let r = i._$litPart$;
  if (r === void 0) {
    const n = (t == null ? void 0 : t.renderBefore) ?? null;
    i._$litPart$ = r = new F(e.insertBefore(I(), n), n, void 0, t ?? {});
  }
  return r._$AI(s), r;
};
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const E = globalThis;
class y extends T {
  constructor() {
    super(...arguments), this.renderOptions = { host: this }, this._$Do = void 0;
  }
  createRenderRoot() {
    var t;
    const e = super.createRenderRoot();
    return (t = this.renderOptions).renderBefore ?? (t.renderBefore = e.firstChild), e;
  }
  update(e) {
    const t = this.render();
    this.hasUpdated || (this.renderOptions.isConnected = this.isConnected), super.update(e), this._$Do = Be(t, this.renderRoot, this.renderOptions);
  }
  connectedCallback() {
    var e;
    super.connectedCallback(), (e = this._$Do) == null || e.setConnected(!0);
  }
  disconnectedCallback() {
    var e;
    super.disconnectedCallback(), (e = this._$Do) == null || e.setConnected(!1);
  }
  render() {
    return D;
  }
}
var $e;
y._$litElement$ = !0, y.finalized = !0, ($e = E.litElementHydrateSupport) == null || $e.call(E, { LitElement: y });
const se = E.litElementPolyfillSupport;
se == null || se({ LitElement: y });
(E.litElementVersions ?? (E.litElementVersions = [])).push("4.2.2");
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const W = (s) => (e, t) => {
  t !== void 0 ? t.addInitializer(() => {
    customElements.define(s, e);
  }) : customElements.define(s, e);
};
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
const Fe = { attribute: !0, type: String, converter: J, reflect: !1, hasChanged: oe }, We = (s = Fe, e, t) => {
  const { kind: i, metadata: r } = t;
  let n = globalThis.litPropertyMetadata.get(r);
  if (n === void 0 && globalThis.litPropertyMetadata.set(r, n = /* @__PURE__ */ new Map()), i === "setter" && ((s = Object.create(s)).wrapped = !0), n.set(t.name, s), i === "accessor") {
    const { name: o } = t;
    return { set(l) {
      const a = e.get.call(this);
      e.set.call(this, l), this.requestUpdate(o, a, s, !0, l);
    }, init(l) {
      return l !== void 0 && this.C(o, void 0, s, l), l;
    } };
  }
  if (i === "setter") {
    const { name: o } = t;
    return function(l) {
      const a = this[o];
      e.call(this, l), this.requestUpdate(o, a, s, !0, l);
    };
  }
  throw Error("Unsupported decorator location: " + i);
};
function f(s) {
  return (e, t) => typeof t == "object" ? We(s, e, t) : ((i, r, n) => {
    const o = r.hasOwnProperty(n);
    return r.constructor.createProperty(n, i), o ? Object.getOwnPropertyDescriptor(r, n) : void 0;
  })(s, e, t);
}
/**
 * @license
 * Copyright 2017 Google LLC
 * SPDX-License-Identifier: BSD-3-Clause
 */
function u(s) {
  return f({ ...s, state: !0, attribute: !1 });
}
function Ae(s, e) {
  const t = new WebSocket(s);
  return t.onmessage = (i) => {
    var r, n, o, l;
    try {
      const a = JSON.parse(i.data);
      ((n = (r = a.type) == null ? void 0 : r.startsWith) != null && n.call(r, "process.") || (l = (o = a.channel) == null ? void 0 : o.startsWith) != null && l.call(o, "process.")) && e(a);
    } catch {
    }
  }, t;
}
class G {
  constructor(e = "") {
    this.baseUrl = e;
  }
  get base() {
    return `${this.baseUrl}/api/process`;
  }
  async request(e, t) {
    var n;
    const r = await (await fetch(`${this.base}${e}`, t)).json();
    if (!r.success)
      throw new Error(((n = r.error) == null ? void 0 : n.message) ?? "Request failed");
    return r.data;
  }
  /** List all alive daemons from the registry. */
  listDaemons() {
    return this.request("/daemons");
  }
  /** Get a single daemon entry by code and daemon name. */
  getDaemon(e, t) {
    return this.request(`/daemons/${e}/${t}`);
  }
  /** Stop a daemon (SIGTERM + unregister). */
  stopDaemon(e, t) {
    return this.request(`/daemons/${e}/${t}/stop`, {
      method: "POST"
    });
  }
  /** Check daemon health endpoint. */
  healthCheck(e, t) {
    return this.request(`/daemons/${e}/${t}/health`);
  }
  /** List all managed processes. */
  listProcesses() {
    return this.request("/processes");
  }
  /** Get a single managed process by ID. */
  getProcess(e) {
    return this.request(`/processes/${e}`);
  }
  /** Get the captured stdout/stderr for a managed process by ID. */
  getProcessOutput(e) {
    return this.request(`/processes/${e}/output`);
  }
  /** Kill a managed process by ID. */
  killProcess(e) {
    return this.request(`/processes/${e}/kill`, {
      method: "POST"
    });
  }
  /** Run a process pipeline using the configured runner. */
  runPipeline(e, t) {
    return this.request("/pipelines/run", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ mode: e, specs: t })
    });
  }
}
var Ke = Object.defineProperty, Ve = Object.getOwnPropertyDescriptor, k = (s, e, t, i) => {
  for (var r = i > 1 ? void 0 : i ? Ve(e, t) : e, n = s.length - 1, o; n >= 0; n--)
    (o = s[n]) && (r = (i ? o(e, t, r) : o(r)) || r);
  return i && r && Ke(e, t, r), r;
};
let g = class extends y {
  constructor() {
    super(...arguments), this.apiUrl = "", this.daemons = [], this.loading = !0, this.error = "", this.stopping = /* @__PURE__ */ new Set(), this.checking = /* @__PURE__ */ new Set(), this.healthResults = /* @__PURE__ */ new Map();
  }
  connectedCallback() {
    super.connectedCallback(), this.api = new G(this.apiUrl), this.loadDaemons();
  }
  async loadDaemons() {
    this.loading = !0, this.error = "";
    try {
      this.daemons = await this.api.listDaemons();
    } catch (s) {
      this.error = s.message ?? "Failed to load daemons";
    } finally {
      this.loading = !1;
    }
  }
  daemonKey(s) {
    return `${s.code}/${s.daemon}`;
  }
  async handleStop(s) {
    const e = this.daemonKey(s);
    this.stopping = /* @__PURE__ */ new Set([...this.stopping, e]);
    try {
      await this.api.stopDaemon(s.code, s.daemon), this.dispatchEvent(
        new CustomEvent("daemon-stopped", {
          detail: { code: s.code, daemon: s.daemon },
          bubbles: !0
        })
      ), await this.loadDaemons();
    } catch (t) {
      this.error = t.message ?? "Failed to stop daemon";
    } finally {
      const t = new Set(this.stopping);
      t.delete(e), this.stopping = t;
    }
  }
  async handleHealthCheck(s) {
    const e = this.daemonKey(s);
    this.checking = /* @__PURE__ */ new Set([...this.checking, e]);
    try {
      const t = await this.api.healthCheck(s.code, s.daemon), i = new Map(this.healthResults);
      i.set(e, t), this.healthResults = i;
    } catch (t) {
      this.error = t.message ?? "Health check failed";
    } finally {
      const t = new Set(this.checking);
      t.delete(e), this.checking = t;
    }
  }
  formatDate(s) {
    try {
      return new Date(s).toLocaleDateString("en-GB", {
        day: "numeric",
        month: "short",
        year: "numeric",
        hour: "2-digit",
        minute: "2-digit"
      });
    } catch {
      return s;
    }
  }
  renderHealthBadge(s) {
    const e = this.daemonKey(s);
    if (this.checking.has(e))
      return c`<span class="health-badge checking">Checking\u2026</span>`;
    const t = this.healthResults.get(e);
    return t ? c`<span class="health-badge ${t.healthy ? "healthy" : "unhealthy"}">
        ${t.healthy ? "Healthy" : "Unhealthy"}
      </span>` : s.health ? c`<span class="health-badge unknown">Unchecked</span>` : c`<span class="health-badge unknown">No health endpoint</span>`;
  }
  render() {
    return this.loading ? c`<div class="loading">Loading daemons\u2026</div>` : c`
      ${this.error ? c`<div class="error">${this.error}</div>` : d}
      ${this.daemons.length === 0 ? c`<div class="empty">No daemons registered.</div>` : c`
            <div class="list">
              ${this.daemons.map((s) => {
      const e = this.daemonKey(s);
      return c`
                  <div class="item">
                    <div class="item-info">
                      <div class="item-name">
                        <span class="item-code">${s.code}</span>
                        <span>${s.daemon}</span>
                        ${this.renderHealthBadge(s)}
                      </div>
                      <div class="item-meta">
                        <span class="pid-badge">PID ${s.pid}</span>
                        <span>Started ${this.formatDate(s.started)}</span>
                        ${s.project ? c`<span>${s.project}</span>` : d}
                        ${s.binary ? c`<span>${s.binary}</span>` : d}
                      </div>
                    </div>
                    <div class="item-actions">
                      ${s.health ? c`
                            <button
                              class="health-btn"
                              ?disabled=${this.checking.has(e)}
                              @click=${() => this.handleHealthCheck(s)}
                            >
                              ${this.checking.has(e) ? "Checking…" : "Health"}
                            </button>
                          ` : d}
                      <button
                        class="stop-btn"
                        ?disabled=${this.stopping.has(e)}
                        @click=${() => this.handleStop(s)}
                      >
                        ${this.stopping.has(e) ? "Stopping…" : "Stop"}
                      </button>
                    </div>
                  </div>
                `;
    })}
            </div>
          `}
    `;
  }
};
g.styles = B`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
    }

    .list {
      display: flex;
      flex-direction: column;
      gap: 0.5rem;
    }

    .item {
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      padding: 1rem;
      background: #fff;
      display: flex;
      justify-content: space-between;
      align-items: center;
      transition: box-shadow 0.15s;
    }

    .item:hover {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
    }

    .item-info {
      flex: 1;
    }

    .item-name {
      font-weight: 600;
      font-size: 0.9375rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .item-code {
      font-family: monospace;
      font-size: 0.8125rem;
      colour: #6366f1;
    }

    .item-meta {
      font-size: 0.75rem;
      colour: #6b7280;
      margin-top: 0.25rem;
      display: flex;
      gap: 1rem;
    }

    .pid-badge {
      font-family: monospace;
      background: #f3f4f6;
      padding: 0.0625rem 0.375rem;
      border-radius: 0.25rem;
      font-size: 0.6875rem;
    }

    .health-badge {
      font-size: 0.6875rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .health-badge.healthy {
      background: #dcfce7;
      colour: #166534;
    }

    .health-badge.unhealthy {
      background: #fef2f2;
      colour: #991b1b;
    }

    .health-badge.unknown {
      background: #f3f4f6;
      colour: #6b7280;
    }

    .health-badge.checking {
      background: #fef3c7;
      colour: #92400e;
    }

    .item-actions {
      display: flex;
      gap: 0.5rem;
    }

    button {
      padding: 0.375rem 0.75rem;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      cursor: pointer;
      transition: background 0.15s;
    }

    button.health-btn {
      background: #fff;
      colour: #6366f1;
      border: 1px solid #6366f1;
    }

    button.health-btn:hover {
      background: #eef2ff;
    }

    button.health-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    button.stop-btn {
      background: #fff;
      colour: #dc2626;
      border: 1px solid #dc2626;
    }

    button.stop-btn:hover {
      background: #fef2f2;
    }

    button.stop-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #9ca3af;
      font-size: 0.875rem;
    }

    .loading {
      text-align: center;
      padding: 2rem;
      colour: #6b7280;
    }

    .error {
      colour: #dc2626;
      padding: 0.75rem;
      background: #fef2f2;
      border-radius: 0.375rem;
      font-size: 0.875rem;
      margin-bottom: 1rem;
    }
  `;
k([
  f({ attribute: "api-url" })
], g.prototype, "apiUrl", 2);
k([
  u()
], g.prototype, "daemons", 2);
k([
  u()
], g.prototype, "loading", 2);
k([
  u()
], g.prototype, "error", 2);
k([
  u()
], g.prototype, "stopping", 2);
k([
  u()
], g.prototype, "checking", 2);
k([
  u()
], g.prototype, "healthResults", 2);
g = k([
  W("core-process-daemons")
], g);
var Je = Object.defineProperty, Ze = Object.getOwnPropertyDescriptor, O = (s, e, t, i) => {
  for (var r = i > 1 ? void 0 : i ? Ze(e, t) : e, n = s.length - 1, o; n >= 0; n--)
    (o = s[n]) && (r = (i ? o(e, t, r) : o(r)) || r);
  return i && r && Je(e, t, r), r;
};
let v = class extends y {
  constructor() {
    super(...arguments), this.apiUrl = "", this.selectedId = "", this.processes = [], this.loading = !1, this.error = "", this.killing = /* @__PURE__ */ new Set();
  }
  connectedCallback() {
    super.connectedCallback(), this.api = new G(this.apiUrl), this.loadProcesses();
  }
  async loadProcesses() {
    this.loading = !0, this.error = "";
    try {
      this.processes = await this.api.listProcesses();
    } catch (s) {
      this.error = s.message ?? "Failed to load processes", this.processes = [];
    } finally {
      this.loading = !1;
    }
  }
  handleSelect(s) {
    this.dispatchEvent(
      new CustomEvent("process-selected", {
        detail: { id: s.id },
        bubbles: !0,
        composed: !0
      })
    );
  }
  async handleKill(s) {
    this.killing = /* @__PURE__ */ new Set([...this.killing, s.id]);
    try {
      await this.api.killProcess(s.id), await this.loadProcesses();
    } catch (e) {
      this.error = e.message ?? "Failed to kill process";
    } finally {
      const e = new Set(this.killing);
      e.delete(s.id), this.killing = e;
    }
  }
  formatUptime(s) {
    try {
      const e = Date.now() - new Date(s).getTime(), t = Math.floor(e / 1e3);
      if (t < 60) return `${t}s`;
      const i = Math.floor(t / 60);
      return i < 60 ? `${i}m ${t % 60}s` : `${Math.floor(i / 60)}h ${i % 60}m`;
    } catch {
      return "unknown";
    }
  }
  render() {
    return this.loading ? c`<div class="loading">Loading processes\u2026</div>` : c`
      ${this.error ? c`<div class="error">${this.error}</div>` : d}
      ${this.processes.length === 0 ? c`
            <div class="info-notice">
              Managed processes are loaded from the process REST API.
            </div>
            <div class="empty">No managed processes.</div>
          ` : c`
            <div class="list">
              ${this.processes.map(
      (s) => {
        var e;
        return c`
                  <div
                    class="item ${this.selectedId === s.id ? "selected" : ""}"
                    @click=${() => this.handleSelect(s)}
                  >
                    <div class="item-info">
                      <div class="item-command">
                        <span>${s.command} ${((e = s.args) == null ? void 0 : e.join(" ")) ?? ""}</span>
                        <span class="status-badge ${s.status}">${s.status}</span>
                      </div>
                      <div class="item-meta">
                        <span class="pid-badge">PID ${s.pid}</span>
                        <span>${s.id}</span>
                        ${s.dir ? c`<span>${s.dir}</span>` : d}
                        ${s.status === "running" ? c`<span>Up ${this.formatUptime(s.startedAt)}</span>` : d}
                        ${s.status === "exited" ? c`<span class="exit-code ${s.exitCode !== 0 ? "nonzero" : ""}">
                              exit ${s.exitCode}
                            </span>` : d}
                      </div>
                    </div>
                    ${s.status === "running" ? c`
                          <div class="item-actions">
                            <button
                              class="kill-btn"
                              ?disabled=${this.killing.has(s.id)}
                              @click=${(t) => {
          t.stopPropagation(), this.handleKill(s);
        }}
                            >
                              ${this.killing.has(s.id) ? "Killing…" : "Kill"}
                            </button>
                          </div>
                        ` : d}
                  </div>
                `;
      }
    )}
            </div>
          `}
    `;
  }
};
v.styles = B`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
    }

    .list {
      display: flex;
      flex-direction: column;
      gap: 0.375rem;
    }

    .item {
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      padding: 0.75rem 1rem;
      background: #fff;
      display: flex;
      justify-content: space-between;
      align-items: center;
      cursor: pointer;
      transition: box-shadow 0.15s, border-colour 0.15s;
    }

    .item:hover {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
    }

    .item.selected {
      border-colour: #6366f1;
      box-shadow: 0 0 0 1px #6366f1;
    }

    .item-info {
      flex: 1;
    }

    .item-command {
      font-weight: 600;
      font-size: 0.9375rem;
      font-family: monospace;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .item-meta {
      font-size: 0.75rem;
      colour: #6b7280;
      margin-top: 0.25rem;
      display: flex;
      gap: 1rem;
    }

    .status-badge {
      font-size: 0.6875rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .status-badge.running {
      background: #dbeafe;
      colour: #1e40af;
    }

    .status-badge.pending {
      background: #fef3c7;
      colour: #92400e;
    }

    .status-badge.exited {
      background: #dcfce7;
      colour: #166534;
    }

    .status-badge.failed {
      background: #fef2f2;
      colour: #991b1b;
    }

    .status-badge.killed {
      background: #fce7f3;
      colour: #9d174d;
    }

    .exit-code {
      font-family: monospace;
      font-size: 0.6875rem;
      background: #f3f4f6;
      padding: 0.0625rem 0.375rem;
      border-radius: 0.25rem;
    }

    .exit-code.nonzero {
      background: #fef2f2;
      colour: #991b1b;
    }

    .pid-badge {
      font-family: monospace;
      background: #f3f4f6;
      padding: 0.0625rem 0.375rem;
      border-radius: 0.25rem;
      font-size: 0.6875rem;
    }

    .item-actions {
      display: flex;
      gap: 0.5rem;
    }

    button.kill-btn {
      padding: 0.375rem 0.75rem;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      cursor: pointer;
      transition: background 0.15s;
      background: #fff;
      colour: #dc2626;
      border: 1px solid #dc2626;
    }

    button.kill-btn:hover {
      background: #fef2f2;
    }

    button.kill-btn:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #9ca3af;
      font-size: 0.875rem;
    }

    .loading {
      text-align: center;
      padding: 2rem;
      colour: #6b7280;
    }

    .error {
      colour: #dc2626;
      padding: 0.75rem;
      background: #fef2f2;
      border-radius: 0.375rem;
      font-size: 0.875rem;
      margin-bottom: 1rem;
    }

    .info-notice {
      padding: 0.75rem;
      background: #eff6ff;
      border: 1px solid #bfdbfe;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      colour: #1e40af;
      margin-bottom: 1rem;
    }
  `;
O([
  f({ attribute: "api-url" })
], v.prototype, "apiUrl", 2);
O([
  f({ attribute: "selected-id" })
], v.prototype, "selectedId", 2);
O([
  u()
], v.prototype, "processes", 2);
O([
  u()
], v.prototype, "loading", 2);
O([
  u()
], v.prototype, "error", 2);
O([
  u()
], v.prototype, "killing", 2);
v = O([
  W("core-process-list")
], v);
var Ge = Object.defineProperty, Qe = Object.getOwnPropertyDescriptor, S = (s, e, t, i) => {
  for (var r = i > 1 ? void 0 : i ? Qe(e, t) : e, n = s.length - 1, o; n >= 0; n--)
    (o = s[n]) && (r = (i ? o(e, t, r) : o(r)) || r);
  return i && r && Ge(e, t, r), r;
};
let b = class extends y {
  constructor() {
    super(...arguments), this.apiUrl = "", this.wsUrl = "", this.processId = "", this.lines = [], this.autoScroll = !0, this.connected = !1, this.loadingSnapshot = !1, this.ws = null, this.api = new G(this.apiUrl), this.syncToken = 0;
  }
  connectedCallback() {
    super.connectedCallback(), this.syncSources();
  }
  disconnectedCallback() {
    super.disconnectedCallback(), this.disconnect();
  }
  updated(s) {
    s.has("apiUrl") && (this.api = new G(this.apiUrl)), (s.has("processId") || s.has("wsUrl") || s.has("apiUrl")) && this.syncSources(), this.autoScroll && this.scrollToBottom();
  }
  syncSources() {
    this.disconnect(), this.lines = [], this.processId && this.loadSnapshotAndConnect();
  }
  async loadSnapshotAndConnect() {
    const s = ++this.syncToken;
    if (this.processId) {
      if (this.apiUrl) {
        this.loadingSnapshot = !0;
        try {
          const e = await this.api.getProcessOutput(this.processId);
          if (s !== this.syncToken)
            return;
          const t = this.linesFromOutput(e);
          t.length > 0 && (this.lines = t);
        } catch {
        } finally {
          s === this.syncToken && (this.loadingSnapshot = !1);
        }
      }
      s === this.syncToken && this.wsUrl && this.connect();
    }
  }
  linesFromOutput(s) {
    if (!s)
      return [];
    const t = s.replace(/\r\n/g, `
`).split(`
`);
    return t.length > 0 && t[t.length - 1] === "" && t.pop(), t.map((i) => ({
      text: i,
      stream: "stdout",
      timestamp: Date.now()
    }));
  }
  connect() {
    this.ws = Ae(this.wsUrl, (s) => {
      const e = s.data;
      if (!e) return;
      (s.channel ?? s.type ?? "") === "process.output" && e.id === this.processId && (this.lines = [
        ...this.lines,
        {
          text: e.line ?? "",
          stream: e.stream === "stderr" ? "stderr" : "stdout",
          timestamp: Date.now()
        }
      ]);
    }), this.ws.onopen = () => {
      this.connected = !0;
    }, this.ws.onclose = () => {
      this.connected = !1;
    };
  }
  disconnect() {
    this.ws && (this.ws.close(), this.ws = null), this.connected = !1;
  }
  handleClear() {
    this.lines = [];
  }
  handleAutoScrollToggle() {
    this.autoScroll = !this.autoScroll;
  }
  scrollToBottom() {
    var e;
    const s = (e = this.shadowRoot) == null ? void 0 : e.querySelector(".output-body");
    s && (s.scrollTop = s.scrollHeight);
  }
  render() {
    return this.processId ? c`
      <div class="output-header">
        <span class="output-title">Output: ${this.processId}</span>
        <div class="output-actions">
          <label class="auto-scroll-toggle">
            <input
              type="checkbox"
              ?checked=${this.autoScroll}
              @change=${this.handleAutoScrollToggle}
            />
            Auto-scroll
          </label>
          <button class="clear-btn" @click=${this.handleClear}>Clear</button>
        </div>
      </div>
      <div class="output-body">
        ${this.loadingSnapshot && this.lines.length === 0 ? c`<div class="waiting">Loading snapshot\u2026</div>` : this.lines.length === 0 ? c`<div class="waiting">Waiting for output\u2026</div>` : this.lines.map(
      (s) => c`
                <div class="line ${s.stream}">
                  <span class="stream-tag">${s.stream}</span>${s.text}
                </div>
              `
    )}
      </div>
    ` : c`<div class="empty">Select a process to view its output.</div>`;
  }
};
b.styles = B`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
      margin-top: 0.75rem;
    }

    .output-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.5rem 0.75rem;
      background: #1e1e1e;
      border-radius: 0.5rem 0.5rem 0 0;
      colour: #d4d4d4;
      font-size: 0.75rem;
    }

    .output-title {
      font-weight: 600;
    }

    .output-actions {
      display: flex;
      gap: 0.5rem;
    }

    button.clear-btn {
      padding: 0.25rem 0.5rem;
      border-radius: 0.25rem;
      font-size: 0.6875rem;
      cursor: pointer;
      background: #333;
      colour: #d4d4d4;
      border: 1px solid #555;
      transition: background 0.15s;
    }

    button.clear-btn:hover {
      background: #444;
    }

    .auto-scroll-toggle {
      display: flex;
      align-items: center;
      gap: 0.25rem;
      font-size: 0.6875rem;
      colour: #d4d4d4;
      cursor: pointer;
    }

    .auto-scroll-toggle input {
      cursor: pointer;
    }

    .output-body {
      background: #1e1e1e;
      border-radius: 0 0 0.5rem 0.5rem;
      padding: 0.5rem 0.75rem;
      max-height: 24rem;
      overflow-y: auto;
      font-family: 'SF Mono', 'Monaco', 'Menlo', 'Consolas', monospace;
      font-size: 0.8125rem;
      line-height: 1.5;
    }

    .line {
      white-space: pre-wrap;
      word-break: break-all;
    }

    .line.stdout {
      colour: #d4d4d4;
    }

    .line.stderr {
      colour: #f87171;
    }

    .stream-tag {
      display: inline-block;
      width: 3rem;
      font-size: 0.625rem;
      font-weight: 600;
      text-transform: uppercase;
      opacity: 0.5;
      margin-right: 0.5rem;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #6b7280;
      font-size: 0.8125rem;
    }

    .waiting {
      colour: #9ca3af;
      font-style: italic;
      padding: 1rem;
      text-align: center;
      font-size: 0.8125rem;
    }
  `;
S([
  f({ attribute: "api-url" })
], b.prototype, "apiUrl", 2);
S([
  f({ attribute: "ws-url" })
], b.prototype, "wsUrl", 2);
S([
  f({ attribute: "process-id" })
], b.prototype, "processId", 2);
S([
  u()
], b.prototype, "lines", 2);
S([
  u()
], b.prototype, "autoScroll", 2);
S([
  u()
], b.prototype, "connected", 2);
S([
  u()
], b.prototype, "loadingSnapshot", 2);
b = S([
  W("core-process-output")
], b);
var Xe = Object.defineProperty, Ye = Object.getOwnPropertyDescriptor, X = (s, e, t, i) => {
  for (var r = i > 1 ? void 0 : i ? Ye(e, t) : e, n = s.length - 1, o; n >= 0; n--)
    (o = s[n]) && (r = (i ? o(e, t, r) : o(r)) || r);
  return i && r && Xe(e, t, r), r;
};
let H = class extends y {
  constructor() {
    super(...arguments), this.apiUrl = "", this.result = null, this.expandedOutputs = /* @__PURE__ */ new Set();
  }
  connectedCallback() {
    super.connectedCallback(), this.loadResults();
  }
  async loadResults() {
  }
  toggleOutput(s) {
    const e = new Set(this.expandedOutputs);
    e.has(s) ? e.delete(s) : e.add(s), this.expandedOutputs = e;
  }
  formatDuration(s) {
    return s < 1e6 ? `${(s / 1e3).toFixed(0)}µs` : s < 1e9 ? `${(s / 1e6).toFixed(0)}ms` : `${(s / 1e9).toFixed(2)}s`;
  }
  resultStatus(s) {
    return s.skipped ? "skipped" : s.passed ? "passed" : "failed";
  }
  render() {
    if (!this.result)
      return c`
        <div class="info-notice">
          Pass pipeline results via the <code>result</code> property.
        </div>
        <div class="empty">No pipeline results.</div>
      `;
    const { results: s, duration: e, passed: t, failed: i, skipped: r, success: n } = this.result;
    return c`
      <div class="summary">
        <span class="overall-badge ${n ? "success" : "failure"}">
          ${n ? "Passed" : "Failed"}
        </span>
        <div class="summary-stat">
          <span class="summary-value passed">${t}</span>
          <span class="summary-label">Passed</span>
        </div>
        <div class="summary-stat">
          <span class="summary-value failed">${i}</span>
          <span class="summary-label">Failed</span>
        </div>
        <div class="summary-stat">
          <span class="summary-value skipped">${r}</span>
          <span class="summary-label">Skipped</span>
        </div>
        <span class="summary-duration">${this.formatDuration(e)}</span>
      </div>

      <div class="list">
        ${s.map(
      (o) => c`
            <div class="spec">
              <div class="spec-header">
                <div class="spec-name">
                  <span>${o.name}</span>
                  <span class="result-badge ${this.resultStatus(o)}">${this.resultStatus(o)}</span>
                </div>
                <span class="duration">${this.formatDuration(o.duration)}</span>
              </div>
              <div class="spec-meta">
                ${o.exitCode !== 0 && !o.skipped ? c`<span>exit ${o.exitCode}</span>` : d}
              </div>
              ${o.error ? c`<div class="spec-error">${o.error}</div>` : d}
              ${o.output ? c`
                    <button class="toggle-output" @click=${() => this.toggleOutput(o.name)}>
                      ${this.expandedOutputs.has(o.name) ? "Hide output" : "Show output"}
                    </button>
                    ${this.expandedOutputs.has(o.name) ? c`<div class="spec-output">${o.output}</div>` : d}
                  ` : d}
            </div>
          `
    )}
      </div>
    `;
  }
};
H.styles = B`
    :host {
      display: block;
      font-family: system-ui, -apple-system, sans-serif;
    }

    .summary {
      display: flex;
      gap: 1rem;
      padding: 0.75rem 1rem;
      background: #fff;
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      margin-bottom: 1rem;
      align-items: center;
    }

    .summary-stat {
      display: flex;
      flex-direction: column;
      align-items: center;
    }

    .summary-value {
      font-weight: 700;
      font-size: 1.25rem;
    }

    .summary-label {
      font-size: 0.6875rem;
      colour: #6b7280;
      text-transform: uppercase;
      letter-spacing: 0.025em;
    }

    .summary-value.passed {
      colour: #166534;
    }

    .summary-value.failed {
      colour: #991b1b;
    }

    .summary-value.skipped {
      colour: #92400e;
    }

    .summary-duration {
      margin-left: auto;
      font-size: 0.8125rem;
      colour: #6b7280;
    }

    .overall-badge {
      font-size: 0.75rem;
      padding: 0.25rem 0.75rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .overall-badge.success {
      background: #dcfce7;
      colour: #166534;
    }

    .overall-badge.failure {
      background: #fef2f2;
      colour: #991b1b;
    }

    .list {
      display: flex;
      flex-direction: column;
      gap: 0.375rem;
    }

    .spec {
      border: 1px solid #e5e7eb;
      border-radius: 0.5rem;
      padding: 0.75rem 1rem;
      background: #fff;
      transition: box-shadow 0.15s;
    }

    .spec:hover {
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
    }

    .spec-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
    }

    .spec-name {
      font-weight: 600;
      font-size: 0.9375rem;
      display: flex;
      align-items: center;
      gap: 0.5rem;
    }

    .spec-meta {
      font-size: 0.75rem;
      colour: #6b7280;
      margin-top: 0.25rem;
      display: flex;
      gap: 1rem;
    }

    .result-badge {
      font-size: 0.6875rem;
      padding: 0.125rem 0.5rem;
      border-radius: 1rem;
      font-weight: 600;
    }

    .result-badge.passed {
      background: #dcfce7;
      colour: #166534;
    }

    .result-badge.failed {
      background: #fef2f2;
      colour: #991b1b;
    }

    .result-badge.skipped {
      background: #fef3c7;
      colour: #92400e;
    }

    .duration {
      font-family: monospace;
      font-size: 0.75rem;
      colour: #6b7280;
    }

    .deps {
      font-size: 0.6875rem;
      colour: #9ca3af;
    }

    .spec-output {
      margin-top: 0.5rem;
      padding: 0.5rem 0.75rem;
      background: #1e1e1e;
      border-radius: 0.375rem;
      font-family: 'SF Mono', 'Monaco', 'Menlo', 'Consolas', monospace;
      font-size: 0.75rem;
      line-height: 1.5;
      colour: #d4d4d4;
      white-space: pre-wrap;
      word-break: break-all;
      max-height: 12rem;
      overflow-y: auto;
    }

    .spec-error {
      margin-top: 0.375rem;
      font-size: 0.75rem;
      colour: #dc2626;
    }

    .toggle-output {
      font-size: 0.6875rem;
      colour: #6366f1;
      cursor: pointer;
      background: none;
      border: none;
      padding: 0;
      margin-top: 0.375rem;
    }

    .toggle-output:hover {
      text-decoration: underline;
    }

    .empty {
      text-align: center;
      padding: 2rem;
      colour: #9ca3af;
      font-size: 0.875rem;
    }

    .info-notice {
      padding: 0.75rem;
      background: #eff6ff;
      border: 1px solid #bfdbfe;
      border-radius: 0.375rem;
      font-size: 0.8125rem;
      colour: #1e40af;
      margin-bottom: 1rem;
    }
  `;
X([
  f({ attribute: "api-url" })
], H.prototype, "apiUrl", 2);
X([
  f({ type: Object })
], H.prototype, "result", 2);
X([
  u()
], H.prototype, "expandedOutputs", 2);
H = X([
  W("core-process-runner")
], H);
var et = Object.defineProperty, tt = Object.getOwnPropertyDescriptor, z = (s, e, t, i) => {
  for (var r = i > 1 ? void 0 : i ? tt(e, t) : e, n = s.length - 1, o; n >= 0; n--)
    (o = s[n]) && (r = (i ? o(e, t, r) : o(r)) || r);
  return i && r && et(e, t, r), r;
};
let _ = class extends y {
  constructor() {
    super(...arguments), this.apiUrl = "", this.wsUrl = "", this.activeTab = "daemons", this.wsConnected = !1, this.lastEvent = "", this.selectedProcessId = "", this.ws = null, this.tabs = [
      { id: "daemons", label: "Daemons" },
      { id: "processes", label: "Processes" },
      { id: "pipelines", label: "Pipelines" }
    ];
  }
  connectedCallback() {
    super.connectedCallback(), this.wsUrl && this.connectWs();
  }
  disconnectedCallback() {
    super.disconnectedCallback(), this.ws && (this.ws.close(), this.ws = null);
  }
  connectWs() {
    this.ws = Ae(this.wsUrl, (s) => {
      this.lastEvent = s.channel ?? s.type ?? "", this.requestUpdate();
    }), this.ws.onopen = () => {
      this.wsConnected = !0;
    }, this.ws.onclose = () => {
      this.wsConnected = !1;
    };
  }
  handleTabClick(s) {
    this.activeTab = s;
  }
  handleRefresh() {
    var e;
    const s = (e = this.shadowRoot) == null ? void 0 : e.querySelector(".content");
    if (s) {
      const t = s.firstElementChild;
      t && "loadDaemons" in t ? t.loadDaemons() : t && "loadProcesses" in t ? t.loadProcesses() : t && "loadResults" in t && t.loadResults();
    }
  }
  handleProcessSelected(s) {
    this.selectedProcessId = s.detail.id;
  }
  renderContent() {
    switch (this.activeTab) {
      case "daemons":
        return c`<core-process-daemons api-url=${this.apiUrl}></core-process-daemons>`;
      case "processes":
        return c`
          <core-process-list
            api-url=${this.apiUrl}
            @process-selected=${this.handleProcessSelected}
          ></core-process-list>
          ${this.selectedProcessId ? c`<core-process-output
                api-url=${this.apiUrl}
                ws-url=${this.wsUrl}
                process-id=${this.selectedProcessId}
              ></core-process-output>` : d}
        `;
      case "pipelines":
        return c`<core-process-runner api-url=${this.apiUrl}></core-process-runner>`;
      default:
        return d;
    }
  }
  render() {
    const s = this.wsUrl ? this.wsConnected ? "connected" : "disconnected" : "idle";
    return c`
      <div class="header">
        <span class="title">Process Manager</span>
        <button class="refresh-btn" @click=${this.handleRefresh}>Refresh</button>
      </div>

      <div class="tabs">
        ${this.tabs.map(
      (e) => c`
            <button
              class="tab ${this.activeTab === e.id ? "active" : ""}"
              @click=${() => this.handleTabClick(e.id)}
            >
              ${e.label}
            </button>
          `
    )}
      </div>

      <div class="content">${this.renderContent()}</div>

      <div class="footer">
        <div class="ws-status">
          <span class="ws-dot ${s}"></span>
          <span>${s === "connected" ? "Connected" : s === "disconnected" ? "Disconnected" : "No WebSocket"}</span>
        </div>
        ${this.lastEvent ? c`<span>Last: ${this.lastEvent}</span>` : d}
      </div>
    `;
  }
};
_.styles = B`
    :host {
      display: flex;
      flex-direction: column;
      font-family: system-ui, -apple-system, sans-serif;
      height: 100%;
      background: #fafafa;
    }

    /* H — Header */
    .header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.75rem 1rem;
      background: #fff;
      border-bottom: 1px solid #e5e7eb;
    }

    .title {
      font-weight: 700;
      font-size: 1rem;
      colour: #111827;
    }

    .refresh-btn {
      padding: 0.375rem 0.75rem;
      border: 1px solid #d1d5db;
      border-radius: 0.375rem;
      background: #fff;
      font-size: 0.8125rem;
      cursor: pointer;
      transition: background 0.15s;
    }

    .refresh-btn:hover {
      background: #f3f4f6;
    }

    /* H-L — Tabs */
    .tabs {
      display: flex;
      gap: 0;
      background: #fff;
      border-bottom: 1px solid #e5e7eb;
      padding: 0 1rem;
    }

    .tab {
      padding: 0.625rem 1rem;
      font-size: 0.8125rem;
      font-weight: 500;
      colour: #6b7280;
      cursor: pointer;
      border-bottom: 2px solid transparent;
      transition: all 0.15s;
      background: none;
      border-top: none;
      border-left: none;
      border-right: none;
    }

    .tab:hover {
      colour: #374151;
    }

    .tab.active {
      colour: #6366f1;
      border-bottom-colour: #6366f1;
    }

    /* C — Content */
    .content {
      flex: 1;
      padding: 1rem;
      overflow-y: auto;
    }

    /* F — Footer / Status bar */
    .footer {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 0.5rem 1rem;
      background: #fff;
      border-top: 1px solid #e5e7eb;
      font-size: 0.75rem;
      colour: #9ca3af;
    }

    .ws-status {
      display: flex;
      align-items: center;
      gap: 0.375rem;
    }

    .ws-dot {
      width: 0.5rem;
      height: 0.5rem;
      border-radius: 50%;
    }

    .ws-dot.connected {
      background: #22c55e;
    }

    .ws-dot.disconnected {
      background: #ef4444;
    }

    .ws-dot.idle {
      background: #d1d5db;
    }
  `;
z([
  f({ attribute: "api-url" })
], _.prototype, "apiUrl", 2);
z([
  f({ attribute: "ws-url" })
], _.prototype, "wsUrl", 2);
z([
  u()
], _.prototype, "activeTab", 2);
z([
  u()
], _.prototype, "wsConnected", 2);
z([
  u()
], _.prototype, "lastEvent", 2);
z([
  u()
], _.prototype, "selectedProcessId", 2);
_ = z([
  W("core-process-panel")
], _);
export {
  G as ProcessApi,
  g as ProcessDaemons,
  v as ProcessList,
  b as ProcessOutput,
  _ as ProcessPanel,
  H as ProcessRunner,
  Ae as connectProcessEvents
};

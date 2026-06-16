import { useRef } from "react";
import { COLORS, FONTS, useScrollReveal } from "@/pages/LandingPage";

// --- YAML Content ---

const K8S_YAML = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: production
  labels:
    app: my-app
    component: web
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
      component: web
  template:
    metadata:
      labels:
        app: my-app
        component: web
    spec:
      containers:
      - name: web
        image: ghcr.io/acme/web:2.1
        ports:
        - containerPort: 3000
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /healthz
            port: 3000
          initialDelaySeconds: 10
          periodSeconds: 15
        readinessProbe:
          httpGet:
            path: /ready
            port: 3000
          initialDelaySeconds: 5
          periodSeconds: 10
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: db-credentials
              key: url
        - name: REDIS_URL
          valueFrom:
            configMapKeyRef:
              name: app-config
              key: redis-url
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: production
spec:
  selector:
    app: my-app
    component: web
  ports:
  - port: 80
    targetPort: 3000
  type: ClusterIP
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web
  namespace: production
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - app.example.com
    secretName: web-tls
  rules:
  - host: app.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: web
            port:
              number: 80
---
apiVersion: v1
kind: Secret
metadata:
  name: db-credentials
  namespace: production
type: Opaque
data:
  url: cG9zdGdyZXM6Ly91c2VyOnBhc3N...
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: production
data:
  redis-url: redis://redis:6379
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: web
  namespace: production
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: web
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70`;

const MOZZA_RECIPE = `name: my-app

slices:
  web:
    image: ghcr.io/acme/web:2.1
    port: 3000
    public: true
    replicas: 3
    health: /healthz
    needs: [db, cache]

  db:
    image: postgres:16-alpine

  cache:
    image: redis:7-alpine`;

// --- Section ---

export function RecipeComparison() {
  const [ref, visible] = useScrollReveal();
  const scrollRef = useRef<HTMLPreElement>(null);

  const k8sLineCount = K8S_YAML.split("\n").length;
  const mozzaLineCount = MOZZA_RECIPE.split("\n").length;

  return (
    <section
      ref={ref}
      style={{
        padding: "100px 24px",
        background: COLORS.bgSubtle,
        borderTop: `1px solid ${COLORS.border}`,
        borderBottom: `1px solid ${COLORS.border}`,
      }}
    >
      <div style={{ maxWidth: 1200, margin: "0 auto" }}>
        {/* Header */}
        <div style={{ textAlign: "center", marginBottom: 64 }}>
          <h2
            style={{
              fontSize: "clamp(28px, 4vw, 40px)",
              fontWeight: 700,
              letterSpacing: -0.8,
              margin: "0 0 12px 0",
              fontFamily: FONTS.sans,
              color: COLORS.text,
              opacity: visible ? 1 : 0,
              transform: visible ? "translateY(0)" : "translateY(20px)",
              transition: "opacity 0.6s ease, transform 0.6s ease",
            }}
          >
            {k8sLineCount} lines of YAML vs{" "}
            <span style={{ color: COLORS.accent }}>{mozzaLineCount} lines</span>
          </h2>
          <p
            style={{
              fontSize: 16,
              color: COLORS.textMuted,
              margin: 0,
              fontFamily: FONTS.sans,
              opacity: visible ? 1 : 0,
              transform: visible ? "translateY(0)" : "translateY(20px)",
              transition: "opacity 0.6s ease 0.1s, transform 0.6s ease 0.1s",
            }}
          >
            Same result. A fraction of the complexity.
          </p>
        </div>

        {/* Comparison panels */}
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "1fr 1fr",
            gap: 24,
            opacity: visible ? 1 : 0,
            transform: visible ? "translateY(0)" : "translateY(24px)",
            transition: "opacity 0.6s ease 0.2s, transform 0.6s ease 0.2s",
          }}
        >
          {/* K8s panel */}
          <div
            style={{
              background: COLORS.terminal,
              borderRadius: 12,
              border: `1px solid ${COLORS.border}`,
              overflow: "hidden",
              minWidth: 0,
            }}
          >
            <div
              style={{
                padding: "12px 20px",
                borderBottom: `1px solid ${COLORS.border}`,
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <div style={{ display: "flex", gap: 6 }}>
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#f85149" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#d29922" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#3fb950" }} />
                </div>
                <span style={{ fontSize: 12, color: COLORS.textDim, fontFamily: FONTS.mono }}>
                  kubernetes/
                </span>
              </div>
              <span
                style={{
                  fontSize: 11,
                  color: COLORS.textDim,
                  fontFamily: FONTS.mono,
                  padding: "2px 8px",
                  background: "rgba(248, 81, 73, 0.1)",
                  border: "1px solid rgba(248, 81, 73, 0.2)",
                  borderRadius: 4,
                }}
              >
                {k8sLineCount} lines / 6 files
              </span>
            </div>
            <pre
              ref={scrollRef}
              style={{
                padding: "16px 20px",
                margin: 0,
                fontFamily: FONTS.mono,
                fontSize: 11,
                lineHeight: 1.6,
                color: COLORS.textDim,
                maxHeight: 400,
                overflow: "auto",
                whiteSpace: "pre",
              }}
            >
              {K8S_YAML}
            </pre>
          </div>

          {/* Mozza panel */}
          <div
            style={{
              background: COLORS.terminal,
              borderRadius: 12,
              border: `1px solid ${COLORS.accent}33`,
              overflow: "hidden",
              minWidth: 0,
              boxShadow: `0 0 32px ${COLORS.accentGlow}`,
            }}
          >
            <div
              style={{
                padding: "12px 20px",
                borderBottom: `1px solid ${COLORS.accent}33`,
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
              }}
            >
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <div style={{ display: "flex", gap: 6 }}>
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#f85149" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#d29922" }} />
                  <div style={{ width: 10, height: 10, borderRadius: "50%", background: "#3fb950" }} />
                </div>
                <span style={{ fontSize: 12, color: COLORS.accent, fontFamily: FONTS.mono, fontWeight: 600 }}>
                  my-app.mozza
                </span>
              </div>
              <span
                style={{
                  fontSize: 11,
                  color: COLORS.accent,
                  fontFamily: FONTS.mono,
                  padding: "2px 8px",
                  background: COLORS.accentDim,
                  border: `1px solid ${COLORS.accent}33`,
                  borderRadius: 4,
                }}
              >
                {mozzaLineCount} lines / 1 file
              </span>
            </div>
            <pre
              style={{
                padding: "16px 20px",
                margin: 0,
                fontFamily: FONTS.mono,
                fontSize: 13,
                lineHeight: 1.7,
                color: COLORS.text,
                whiteSpace: "pre",
              }}
            >
              {MOZZA_RECIPE.split("\n").map((line, i) => {
                const isKey = line.includes(":") && !line.trimStart().startsWith("-");
                const isBracket = line.includes("[");
                return (
                  <div key={i}>
                    {isKey ? (
                      <>
                        <span style={{ color: COLORS.accent }}>{line.split(":")[0]}:</span>
                        <span style={{ color: isBracket ? "#79c0ff" : COLORS.text }}>
                          {line.slice(line.indexOf(":") + 1)}
                        </span>
                      </>
                    ) : (
                      <span style={{ color: line.trimStart().startsWith("-") ? "#79c0ff" : COLORS.text }}>
                        {line}
                      </span>
                    )}
                  </div>
                );
              })}
            </pre>
          </div>
        </div>

        {/* Bottom note */}
        <p
          style={{
            textAlign: "center",
            marginTop: 32,
            fontSize: 14,
            color: COLORS.textDim,
            fontFamily: FONTS.sans,
            opacity: visible ? 1 : 0,
            transition: "opacity 0.6s ease 0.4s",
          }}
        >
          Mozza generates the Deployment, Service, Ingress, Secrets, ConfigMap, and HPA for you.
        </p>
      </div>

      {/* Responsive: stack on mobile */}
      <style>{`
        @media (max-width: 768px) {
          section > div > div[style*="grid-template-columns"] {
            grid-template-columns: 1fr !important;
          }
        }
      `}</style>
    </section>
  );
}

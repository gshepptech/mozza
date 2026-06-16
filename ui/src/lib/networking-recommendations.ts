/**
 * Networking recommendation engine for the deployment wizard.
 * Suggests public/private networking and common ports based on
 * the service image and kind.
 */

export interface NetworkRecommendation {
  /** Whether the service should be publicly accessible. */
  isPublic: boolean;
  /** Human-readable reason for the recommendation. */
  reason: string;
  /** Confidence level of the recommendation. */
  confidence: "high" | "medium" | "low";
  /** Warning message if the user overrides the recommendation. */
  overrideWarning?: string;
}

export interface PortRecommendation {
  /** Suggested port number. */
  port: number;
  /** Human-readable label (e.g., "Common port for PostgreSQL"). */
  label: string;
}

const publicPatterns: Array<{ pattern: RegExp; reason: string }> = [
  { pattern: /nginx/i, reason: "Web servers are typically public-facing" },
  { pattern: /frontend/i, reason: "Frontend apps serve users directly" },
  { pattern: /\bweb\b/i, reason: "Web services are typically public-facing" },
  { pattern: /\bui\b/i, reason: "UI services serve users directly" },
  { pattern: /gateway/i, reason: "API gateways handle external traffic" },
];

const privatePatterns: Array<{
  pattern: RegExp;
  reason: string;
  warning: string;
}> = [
  {
    pattern: /postgres/i,
    reason: "Databases should not be exposed to the internet",
    warning:
      "Exposing a database publicly is a security risk. Only do this if you know what you're doing.",
  },
  {
    pattern: /mysql/i,
    reason: "Databases should not be exposed to the internet",
    warning:
      "Exposing a database publicly is a security risk. Only do this if you know what you're doing.",
  },
  {
    pattern: /mongo/i,
    reason: "Databases should not be exposed to the internet",
    warning:
      "Exposing a database publicly is a security risk. Only do this if you know what you're doing.",
  },
  {
    pattern: /redis/i,
    reason: "Cache services should stay internal",
    warning: "Exposing Redis publicly can lead to data breaches.",
  },
  {
    pattern: /memcached/i,
    reason: "Cache services should stay internal",
    warning: "Exposing Memcached publicly can lead to data breaches.",
  },
  {
    pattern: /rabbitmq/i,
    reason: "Message queues should stay internal",
    warning: "Exposing message queues publicly is a security risk.",
  },
  {
    pattern: /kafka/i,
    reason: "Message brokers should stay internal",
    warning: "Exposing Kafka publicly is a security risk.",
  },
];

const apiPatterns: Array<{ pattern: RegExp; reason: string }> = [
  {
    pattern: /\bapi\b/i,
    reason: "API services are often public, but consider using a gateway",
  },
  {
    pattern: /backend/i,
    reason: "Backend services can be public if they serve API requests directly",
  },
  {
    pattern: /server/i,
    reason: "Server processes may need external access depending on your setup",
  },
];

/**
 * Returns a networking recommendation based on the service name, image, and kind.
 */
export function getNetworkingRecommendation(
  sliceName: string,
  image: string,
  kind?: string
): NetworkRecommendation {
  const searchText = `${sliceName} ${image}`;

  // Check database/cache kinds first (highest confidence).
  if (kind === "database" || kind === "cache") {
    const match = privatePatterns.find((p) => p.pattern.test(searchText));
    return {
      isPublic: false,
      reason: match?.reason ?? "Internal services should not be publicly accessible",
      confidence: "high",
      overrideWarning: match?.warning,
    };
  }

  // Check for known private patterns.
  for (const { pattern, reason, warning } of privatePatterns) {
    if (pattern.test(searchText)) {
      return {
        isPublic: false,
        reason,
        confidence: "high",
        overrideWarning: warning,
      };
    }
  }

  // Check for known public patterns.
  for (const { pattern, reason } of publicPatterns) {
    if (pattern.test(searchText)) {
      return { isPublic: true, reason, confidence: "high" };
    }
  }

  // Check web kind.
  if (kind === "web") {
    return {
      isPublic: true,
      reason: "Web services typically serve users directly",
      confidence: "medium",
    };
  }

  // Check API/backend patterns (medium confidence — could go either way).
  for (const { pattern, reason } of apiPatterns) {
    if (pattern.test(searchText)) {
      return { isPublic: true, reason, confidence: "medium" };
    }
  }

  // Default: private (safer).
  return {
    isPublic: false,
    reason: "Private by default — safer for services that don't need external access",
    confidence: "low",
  };
}

/** Known image → port mappings. */
const portMap: Array<{ pattern: RegExp; port: number; label: string }> = [
  { pattern: /nginx/i, port: 80, label: "Common port for Nginx" },
  { pattern: /postgres/i, port: 5432, label: "Common port for PostgreSQL" },
  { pattern: /mysql/i, port: 3306, label: "Common port for MySQL" },
  { pattern: /mongo/i, port: 27017, label: "Common port for MongoDB" },
  { pattern: /redis/i, port: 6379, label: "Common port for Redis" },
  { pattern: /memcached/i, port: 11211, label: "Common port for Memcached" },
  { pattern: /rabbitmq/i, port: 5672, label: "Common port for RabbitMQ" },
  { pattern: /kafka/i, port: 9092, label: "Common port for Kafka" },
  { pattern: /\b3000\b/, port: 3000, label: "Common port for Node.js apps" },
  { pattern: /\b8080\b/, port: 8080, label: "Common port for Java/Go APIs" },
];

/**
 * Returns a port recommendation based on the image name.
 * Returns undefined if no known mapping exists.
 */
export function getPortRecommendation(
  image: string
): PortRecommendation | undefined {
  for (const { pattern, port, label } of portMap) {
    if (pattern.test(image)) {
      return { port, label };
    }
  }
  return undefined;
}

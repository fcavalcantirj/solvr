// Privacy policy, sections 6 to 13 (data retention through contact). Split out of
// app/privacy/page.tsx so every file stays under the 800-line limit; the text is unchanged.
import { Shield, Database, Lock, Clock, Globe, UserCheck, Mail } from "lucide-react";

export function PrivacyLaterSections() {
  return (
    <>
                {/* Section 6: Data Retention */}
                <section id="data-retention" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Clock size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 06
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Data Retention
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-6">
                    <p className="text-muted-foreground leading-relaxed">
                      We retain your data only as long as necessary:
                    </p>

                    <div className="overflow-x-auto">
                      <table className="w-full border border-border text-sm">
                        <thead>
                          <tr className="bg-secondary/50">
                            <th className="text-left p-4 font-mono text-xs font-medium border-b border-border">
                              Data Type
                            </th>
                            <th className="text-left p-4 font-mono text-xs font-medium border-b border-border">
                              Retention Period
                            </th>
                          </tr>
                        </thead>
                        <tbody className="text-muted-foreground">
                          <tr className="border-b border-border">
                            <td className="p-4">Account information</td>
                            <td className="p-4">
                              Until account deletion + 30 days
                            </td>
                          </tr>
                          <tr className="border-b border-border">
                            <td className="p-4">Public contributions</td>
                            <td className="p-4">
                              Indefinitely (part of collective knowledge)
                            </td>
                          </tr>
                          <tr className="border-b border-border">
                            <td className="p-4">API logs</td>
                            <td className="p-4">90 days</td>
                          </tr>
                          <tr className="border-b border-border">
                            <td className="p-4">IP addresses</td>
                            <td className="p-4">30 days (then anonymized)</td>
                          </tr>
                          <tr className="border-b border-border">
                            <td className="p-4">Payment records</td>
                            <td className="p-4">7 years (legal requirement)</td>
                          </tr>
                          <tr>
                            <td className="p-4">Support tickets</td>
                            <td className="p-4">3 years after resolution</td>
                          </tr>
                        </tbody>
                      </table>
                    </div>
                  </div>
                </section>

                {/* Section 7: Your Rights */}
                <section id="your-rights" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <UserCheck size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 07
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Your Rights
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-6">
                    <p className="text-muted-foreground leading-relaxed">
                      Depending on your location, you may have the following
                      rights regarding your personal data:
                    </p>

                    <div className="grid sm:grid-cols-2 gap-4">
                      {[
                        {
                          right: "Access",
                          desc: "Request a copy of all data we hold about you",
                          action: "Settings → Data Export",
                        },
                        {
                          right: "Rectification",
                          desc: "Correct inaccurate or incomplete data",
                          action: "Settings → Profile",
                        },
                        {
                          right: "Erasure",
                          desc: "Request deletion of your personal data",
                          action: "Settings → Delete Account",
                        },
                        {
                          right: "Portability",
                          desc: "Receive your data in a machine-readable format",
                          action: "Settings → Data Export",
                        },
                        {
                          right: "Objection",
                          desc: "Object to certain processing activities",
                          action: "Contact privacy@solvr.dev",
                        },
                        {
                          right: "Restriction",
                          desc: "Request limited processing of your data",
                          action: "Contact privacy@solvr.dev",
                        },
                      ].map((item) => (
                        <div
                          key={item.right}
                          className="p-4 border border-border"
                        >
                          <h4 className="font-mono text-sm mb-1">
                            Right to {item.right}
                          </h4>
                          <p className="text-sm text-muted-foreground mb-3">
                            {item.desc}
                          </p>
                          <p className="font-mono text-[11px] text-muted-foreground">
                            {item.action}
                          </p>
                        </div>
                      ))}
                    </div>

                    <div className="p-4 border-l-2 border-foreground border-t border-border">
                      <p className="text-sm text-muted-foreground leading-relaxed">
                        <span className="font-mono text-foreground">
                          Response Time:
                        </span>{" "}
                        We respond to all privacy requests within 30 days. For
                        complex requests, we may extend this by an additional 60
                        days with notice.
                      </p>
                    </div>
                  </div>
                </section>

                {/* Section 8: Security */}
                <section id="security" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Lock size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 08
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Security Measures
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-6">
                    <p className="text-muted-foreground leading-relaxed">
                      We implement industry-standard security measures to
                      protect your data:
                    </p>

                    <div className="grid sm:grid-cols-2 gap-4">
                      {[
                        {
                          measure: "Encryption",
                          detail: "TLS 1.3 in transit, AES-256 at rest",
                        },
                        {
                          measure: "Authentication",
                          detail: "Bcrypt hashing, optional 2FA, session management",
                        },
                        {
                          measure: "Infrastructure",
                          detail: "SOC 2 compliant hosting, regular penetration testing",
                        },
                        {
                          measure: "Access Control",
                          detail: "Role-based access, audit logging, principle of least privilege",
                        },
                        {
                          measure: "Monitoring",
                          detail: "24/7 threat detection, anomaly alerts, incident response",
                        },
                        {
                          measure: "Backups",
                          detail: "Encrypted daily backups, geo-redundant storage",
                        },
                      ].map((item) => (
                        <div
                          key={item.measure}
                          className="flex gap-3 p-4 border border-border"
                        >
                          <div className="w-2 h-2 bg-foreground mt-2 shrink-0" />
                          <div>
                            <p className="font-mono text-sm mb-1">
                              {item.measure}
                            </p>
                            <p className="text-sm text-muted-foreground">
                              {item.detail}
                            </p>
                          </div>
                        </div>
                      ))}
                    </div>

                    <div className="p-4 border-t border-border pt-6">
                      <p className="font-mono text-xs text-foreground mb-2">
                        SECURITY INCIDENT RESPONSE
                      </p>
                      <p className="text-sm text-muted-foreground">
                        In the event of a data breach, we will notify affected
                        users within 72 hours and relevant authorities as
                        required by law. Our incident response team is available
                        24/7 at security@solvr.dev.
                      </p>
                    </div>
                  </div>
                </section>

                {/* Section 9: Cookies */}
                <section id="cookies" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Database size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 09
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Cookies & Tracking
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-6">
                    <p className="text-muted-foreground leading-relaxed">
                      We use cookies and similar technologies to provide and
                      improve our services:
                    </p>

                    <div className="space-y-4">
                      {[
                        {
                          type: "Essential",
                          purpose: "Authentication, security, preferences",
                          duration: "Session / 1 year",
                          optional: false,
                        },
                        {
                          type: "Functional",
                          purpose: "Remember your settings and preferences",
                          duration: "1 year",
                          optional: false,
                        },
                        {
                          type: "Analytics",
                          purpose: "Understand usage patterns (privacy-focused)",
                          duration: "1 year",
                          optional: true,
                        },
                      ].map((cookie) => (
                        <div
                          key={cookie.type}
                          className="flex flex-col sm:flex-row sm:items-center gap-4 p-4 border border-border"
                        >
                          <div className="sm:w-24">
                            <span
                              className={`font-mono text-xs px-2 py-1 ${cookie.optional ? "bg-secondary" : "bg-foreground text-background"}`}
                            >
                              {cookie.type}
                            </span>
                          </div>
                          <div className="flex-1">
                            <p className="text-sm text-muted-foreground">
                              {cookie.purpose}
                            </p>
                          </div>
                          <div className="sm:w-24 text-left sm:text-right">
                            <span className="font-mono text-xs text-muted-foreground">
                              {cookie.duration}
                            </span>
                          </div>
                        </div>
                      ))}
                    </div>

                    <p className="text-sm text-muted-foreground">
                      You can manage cookie preferences in your browser
                      settings. Note that disabling essential cookies may affect
                      platform functionality.
                    </p>
                  </div>
                </section>

                {/* Section 10: International Transfers */}
                <section id="international" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Globe size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 10
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        International Transfers
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-4 text-muted-foreground leading-relaxed">
                    <p>
                      Solvr operates globally, and your data may be transferred
                      to and processed in countries other than your own. We
                      ensure appropriate safeguards are in place:
                    </p>
                    <ul className="space-y-2">
                      <li className="flex items-start gap-2">
                        <span className="text-foreground mt-1">—</span>
                        Standard Contractual Clauses (SCCs) for EU data transfers
                      </li>
                      <li className="flex items-start gap-2">
                        <span className="text-foreground mt-1">—</span>
                        Data Processing Agreements with all service providers
                      </li>
                      <li className="flex items-start gap-2">
                        <span className="text-foreground mt-1">—</span>
                        Compliance with GDPR, CCPA, and other regional regulations
                      </li>
                    </ul>
                  </div>
                </section>

                {/* Section 11: Children */}
                <section id="children" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Shield size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 11
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Children&apos;s Privacy
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-4 text-muted-foreground leading-relaxed">
                    <p>
                      Solvr is not intended for users under 16 years of age. We
                      do not knowingly collect personal information from
                      children. If you believe a child has provided us with
                      personal data, please contact us at privacy@solvr.dev.
                    </p>
                  </div>
                </section>

                {/* Section 12: Changes */}
                <section id="changes" className="mb-16 scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Clock size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 12
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Policy Changes
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-4 text-muted-foreground leading-relaxed">
                    <p>
                      We may update this Privacy Policy from time to time. We
                      will notify you of significant changes through:
                    </p>
                    <ul className="space-y-2">
                      <li className="flex items-start gap-2">
                        <span className="text-foreground mt-1">—</span>
                        Email notification to your registered address
                      </li>
                      <li className="flex items-start gap-2">
                        <span className="text-foreground mt-1">—</span>
                        Prominent notice on the Platform
                      </li>
                      <li className="flex items-start gap-2">
                        <span className="text-foreground mt-1">—</span>
                        Webhook notification for AI agents (via operator)
                      </li>
                    </ul>
                    <p>
                      Continued use of Solvr after changes take effect
                      constitutes acceptance of the updated policy.
                    </p>
                  </div>
                </section>

                {/* Section 13: Contact */}
                <section id="contact" className="scroll-mt-24">
                  <div className="flex items-start gap-4 mb-6">
                    <div className="w-10 h-10 flex items-center justify-center bg-secondary shrink-0">
                      <Mail size={18} strokeWidth={1.5} />
                    </div>
                    <div>
                      <p className="font-mono text-[11px] tracking-[0.06em] text-muted-foreground mb-1">
                        SECTION 13
                      </p>
                      <h2 className="text-2xl font-light tracking-tight">
                        Contact Us
                      </h2>
                    </div>
                  </div>
                  <div className="pl-0 lg:pl-14 space-y-6">
                    <p className="text-muted-foreground leading-relaxed">
                      For privacy-related questions or to exercise your rights,
                      contact our Privacy Team:
                    </p>

                    <div className="grid sm:grid-cols-2 gap-4">
                      <div className="border-t border-border pt-6">
                        <p className="font-mono text-xs text-muted-foreground mb-2">
                          PRIVACY INQUIRIES
                        </p>
                        <a
                          href="mailto:privacy@solvr.dev"
                          className="font-mono text-sm hover:underline"
                        >
                          privacy@solvr.dev
                        </a>
                      </div>
                      <div className="border-t border-border pt-6">
                        <p className="font-mono text-xs text-muted-foreground mb-2">
                          DATA PROTECTION OFFICER
                        </p>
                        <a
                          href="mailto:dpo@solvr.dev"
                          className="font-mono text-sm hover:underline"
                        >
                          dpo@solvr.dev
                        </a>
                      </div>
                    </div>

                    <div className="p-6 border-t border-border pt-6">
                      <p className="font-mono text-xs text-foreground mb-3">
                        CONTACT
                      </p>
                      <p className="text-sm text-muted-foreground leading-relaxed">
                        Email:{" "}
                        <a
                          href="mailto:privacy@solvr.dev"
                          className="hover:underline"
                        >
                          privacy@solvr.dev
                        </a>
                      </p>
                    </div>
                  </div>
                </section>
    </>
  );
}

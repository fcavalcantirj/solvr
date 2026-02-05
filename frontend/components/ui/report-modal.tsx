'use client';

import { useState } from 'react';
import { X, Flag, Loader2, CheckCircle } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useReport, REPORT_REASONS } from '@/hooks/use-report';
import { ReportReason, ReportTargetType } from '@/lib/api';
import { cn } from '@/lib/utils';

interface ReportModalProps {
  isOpen: boolean;
  onClose: () => void;
  targetType: ReportTargetType;
  targetId: string;
  targetLabel?: string;
}

export function ReportModal({ isOpen, onClose, targetType, targetId, targetLabel }: ReportModalProps) {
  const [selectedReason, setSelectedReason] = useState<ReportReason | null>(null);
  const [details, setDetails] = useState('');
  const [showSuccess, setShowSuccess] = useState(false);

  const { isSubmitting, error, submitReport, clearError } = useReport({
    onSuccess: () => {
      setShowSuccess(true);
      setTimeout(() => {
        setShowSuccess(false);
        onClose();
        setSelectedReason(null);
        setDetails('');
      }, 2000);
    },
  });

  const handleSubmit = async () => {
    if (!selectedReason) return;
    await submitReport(targetType, targetId, selectedReason, details || undefined);
  };

  const handleClose = () => {
    clearError();
    setSelectedReason(null);
    setDetails('');
    setShowSuccess(false);
    onClose();
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-background/80 backdrop-blur-sm" onClick={handleClose} />

      {/* Modal */}
      <div className="relative bg-card border border-border w-full max-w-md mx-4 p-6 shadow-lg">
        {/* Header */}
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <Flag className="w-4 h-4 text-red-500" />
            <h2 className="font-mono text-sm tracking-wider">REPORT {targetLabel?.toUpperCase() || targetType.toUpperCase()}</h2>
          </div>
          <button onClick={handleClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-4 h-4" />
          </button>
        </div>

        {showSuccess ? (
          <div className="flex flex-col items-center py-8">
            <CheckCircle className="w-12 h-12 text-emerald-500 mb-4" />
            <p className="font-mono text-sm text-center">Thank you for your report.</p>
            <p className="font-mono text-xs text-muted-foreground text-center mt-2">
              We&apos;ll review it as soon as possible.
            </p>
          </div>
        ) : (
          <>
            {/* Reason Selection */}
            <div className="space-y-2 mb-6">
              <label className="font-mono text-xs text-muted-foreground">SELECT REASON</label>
              <div className="space-y-2">
                {REPORT_REASONS.map((reason) => (
                  <button
                    key={reason.value}
                    onClick={() => setSelectedReason(reason.value)}
                    className={cn(
                      'w-full p-3 text-left border transition-colors',
                      selectedReason === reason.value
                        ? 'border-foreground bg-secondary'
                        : 'border-border hover:border-foreground/50'
                    )}
                  >
                    <span className="font-mono text-sm font-medium">{reason.label}</span>
                    <p className="font-mono text-xs text-muted-foreground mt-1">{reason.description}</p>
                  </button>
                ))}
              </div>
            </div>

            {/* Optional Details */}
            <div className="space-y-2 mb-6">
              <label className="font-mono text-xs text-muted-foreground">
                ADDITIONAL DETAILS (OPTIONAL)
              </label>
              <textarea
                value={details}
                onChange={(e) => setDetails(e.target.value)}
                placeholder="Provide any additional context..."
                className="w-full h-24 bg-secondary/50 border border-border p-3 font-mono text-sm resize-none focus:outline-none focus:border-foreground placeholder:text-muted-foreground"
                maxLength={500}
              />
              <p className="font-mono text-[10px] text-muted-foreground text-right">
                {details.length}/500
              </p>
            </div>

            {/* Error */}
            {error && (
              <div className="mb-4 p-3 bg-red-500/10 border border-red-500/20 text-red-500 font-mono text-xs">
                {error}
              </div>
            )}

            {/* Actions */}
            <div className="flex justify-end gap-3">
              <Button variant="ghost" onClick={handleClose} disabled={isSubmitting}>
                <span className="font-mono text-xs">CANCEL</span>
              </Button>
              <Button
                onClick={handleSubmit}
                disabled={!selectedReason || isSubmitting}
                className="font-mono text-xs"
              >
                {isSubmitting ? (
                  <>
                    <Loader2 className="w-3 h-3 mr-2 animate-spin" />
                    SUBMITTING...
                  </>
                ) : (
                  'SUBMIT REPORT'
                )}
              </Button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}

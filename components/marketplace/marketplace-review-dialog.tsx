"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Star } from "lucide-react";
import { cn } from "@/lib/utils";
import { useMarketplaceStore } from "@/lib/stores/marketplace-store";
import { toast } from "sonner";

interface Props {
  itemId: string;
}

export function MarketplaceReviewDialog({ itemId }: Props) {
  const { submitReview } = useMarketplaceStore();
  const [open, setOpen] = useState(false);
  const [rating, setRating] = useState(5);
  const [comment, setComment] = useState("");
  const [loading, setLoading] = useState(false);
  const [hovered, setHovered] = useState(0);

  const handleSubmit = async () => {
    setLoading(true);
    try {
      await submitReview(itemId, rating, comment);
      toast.success("Review submitted");
      setOpen(false);
      setComment("");
      setRating(5);
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "Failed to submit review",
      );
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          Write a Review
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle>Write a Review</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div>
            <Label className="mb-2 block">Rating</Label>
            <div className="flex gap-1">
              {Array.from({ length: 5 }).map((_, i) => (
                <Star
                  key={i}
                  className={cn(
                    "w-6 h-6 cursor-pointer transition-colors",
                    i < (hovered || rating)
                      ? "text-yellow-400 fill-yellow-400"
                      : "text-muted-foreground",
                  )}
                  onMouseEnter={() => setHovered(i + 1)}
                  onMouseLeave={() => setHovered(0)}
                  onClick={() => setRating(i + 1)}
                />
              ))}
            </div>
          </div>
          <div>
            <Label htmlFor="review-comment">Comment</Label>
            <Textarea
              id="review-comment"
              value={comment}
              onChange={(e) => setComment(e.target.value)}
              rows={3}
              placeholder="Share your experience..."
            />
          </div>
          <Button
            className="w-full"
            onClick={handleSubmit}
            disabled={loading}
          >
            {loading ? "Submitting..." : "Submit Review"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}

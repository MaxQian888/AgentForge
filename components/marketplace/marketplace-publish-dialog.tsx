"use client";

import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  useMarketplaceStore,
  type CreateItemRequest,
} from "@/lib/stores/marketplace-store";
import { toast } from "sonner";

interface Props {
  open: boolean;
  onClose: () => void;
}

function slugify(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9-]/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
}

export function MarketplacePublishDialog({ open, onClose }: Props) {
  const { publishItem } = useMarketplaceStore();
  const [loading, setLoading] = useState(false);
  const [form, setForm] = useState<CreateItemRequest>({
    type: "plugin",
    slug: "",
    name: "",
    description: "",
    category: "",
    tags: [],
    license: "MIT",
  });

  const handleNameChange = (name: string) => {
    setForm((f) => ({ ...f, name, slug: slugify(name) }));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await publishItem(form);
      toast.success("Item published successfully");
      onClose();
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to publish");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Publish Item</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="pub-type">Type</Label>
              <Select
                value={form.type}
                onValueChange={(v) =>
                  setForm((f) => ({
                    ...f,
                    type: v as CreateItemRequest["type"],
                  }))
                }
              >
                <SelectTrigger id="pub-type">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="plugin">Plugin</SelectItem>
                  <SelectItem value="skill">Skill</SelectItem>
                  <SelectItem value="role">Role</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label htmlFor="pub-license">License</Label>
              <Select
                value={form.license}
                onValueChange={(v) => setForm((f) => ({ ...f, license: v }))}
              >
                <SelectTrigger id="pub-license">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="MIT">MIT</SelectItem>
                  <SelectItem value="Apache-2.0">Apache 2.0</SelectItem>
                  <SelectItem value="GPL-3.0">GPL 3.0</SelectItem>
                  <SelectItem value="Proprietary">Proprietary</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>
          <div>
            <Label htmlFor="pub-name">Name</Label>
            <Input
              id="pub-name"
              value={form.name}
              onChange={(e) => handleNameChange(e.target.value)}
              required
            />
          </div>
          <div>
            <Label htmlFor="pub-slug">Slug</Label>
            <Input
              id="pub-slug"
              value={form.slug}
              onChange={(e) => setForm((f) => ({ ...f, slug: e.target.value }))}
              required
            />
          </div>
          <div>
            <Label htmlFor="pub-desc">Description</Label>
            <Textarea
              id="pub-desc"
              value={form.description}
              onChange={(e) =>
                setForm((f) => ({ ...f, description: e.target.value }))
              }
              rows={3}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <Label htmlFor="pub-cat">Category</Label>
              <Input
                id="pub-cat"
                value={form.category}
                onChange={(e) =>
                  setForm((f) => ({ ...f, category: e.target.value }))
                }
              />
            </div>
            <div>
              <Label htmlFor="pub-repo">Repository URL</Label>
              <Input
                id="pub-repo"
                value={form.repository_url ?? ""}
                onChange={(e) =>
                  setForm((f) => ({
                    ...f,
                    repository_url: e.target.value || undefined,
                  }))
                }
              />
            </div>
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={loading}>
              {loading ? "Publishing..." : "Publish"}
            </Button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}

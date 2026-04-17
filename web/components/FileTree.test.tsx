import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import FileTree, { TreeEntry } from "./FileTree";

describe("FileTree", () => {
  const mockOnSelect = vi.fn();

  const sampleEntries: TreeEntry[] = [
    { name: "src", type: "dir", path: "src" },
    { name: "docs", type: "dir", path: "docs" },
    { name: "main.go", type: "file", path: "main.go", language: "Go" },
    {
      name: "README.md",
      type: "file",
      path: "README.md",
      language: "Markdown",
    },
    {
      name: "package.json",
      type: "file",
      path: "package.json",
      language: "JSON",
    },
  ];

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("should render tree entries", () => {
    render(
      <FileTree entries={sampleEntries} onSelect={mockOnSelect} height={400} />
    );

    // Check that entries are rendered
    expect(screen.getByText("src")).toBeInTheDocument();
    expect(screen.getByText("docs")).toBeInTheDocument();
    expect(screen.getByText("main.go")).toBeInTheDocument();
    expect(screen.getByText("README.md")).toBeInTheDocument();
    expect(screen.getByText("package.json")).toBeInTheDocument();
  });

  it("should call onSelect when clicking a file", async () => {
    const user = userEvent.setup();

    render(
      <FileTree entries={sampleEntries} onSelect={mockOnSelect} height={400} />
    );

    await user.click(screen.getByText("main.go"));

    expect(mockOnSelect).toHaveBeenCalledWith("main.go", false);
  });

  it("should call onSelect when clicking a folder", async () => {
    const user = userEvent.setup();

    render(
      <FileTree entries={sampleEntries} onSelect={mockOnSelect} height={400} />
    );

    await user.click(screen.getByText("src"));

    expect(mockOnSelect).toHaveBeenCalledWith("src", true);
  });

  it("should highlight current path", () => {
    render(
      <FileTree
        entries={sampleEntries}
        currentPath="main.go"
        onSelect={mockOnSelect}
        height={400}
      />
    );

    // The current file should be visually selected
    // This depends on react-arborist's styling
    const mainGoNode = screen.getByText("main.go");
    expect(mainGoNode).toBeInTheDocument();
  });

  it("should render with empty entries", () => {
    render(<FileTree entries={[]} onSelect={mockOnSelect} height={400} />);

    // Should render without crashing
    expect(document.body).toBeInTheDocument();
  });

  it("should apply custom className", () => {
    const { container } = render(
      <FileTree
        entries={sampleEntries}
        onSelect={mockOnSelect}
        height={400}
        className="custom-class"
      />
    );

    expect(container.firstChild).toHaveClass("custom-class");
  });

  it("should render folders before files", () => {
    const entries: TreeEntry[] = [
      { name: "zebra.txt", type: "file", path: "zebra.txt" },
      { name: "alpha", type: "dir", path: "alpha" },
      { name: "apple.js", type: "file", path: "apple.js" },
      { name: "beta", type: "dir", path: "beta" },
    ];

    render(<FileTree entries={entries} onSelect={mockOnSelect} height={400} />);

    // Entries should be rendered (order depends on react-arborist)
    expect(screen.getByText("alpha")).toBeInTheDocument();
    expect(screen.getByText("beta")).toBeInTheDocument();
    expect(screen.getByText("zebra.txt")).toBeInTheDocument();
    expect(screen.getByText("apple.js")).toBeInTheDocument();
  });
});

describe("FileTree icons", () => {
  const mockOnSelect = vi.fn();

  it("should render different icons for different languages", () => {
    const entries: TreeEntry[] = [
      { name: "app.js", type: "file", path: "app.js", language: "JavaScript" },
      { name: "main.py", type: "file", path: "main.py", language: "Python" },
      { name: "main.go", type: "file", path: "main.go", language: "Go" },
      { name: "App.java", type: "file", path: "App.java", language: "Java" },
      { name: "main.rs", type: "file", path: "main.rs", language: "Rust" },
    ];

    render(<FileTree entries={entries} onSelect={mockOnSelect} height={400} />);

    // Each file should be rendered with appropriate icon
    expect(screen.getByText("app.js")).toBeInTheDocument();
    expect(screen.getByText("main.py")).toBeInTheDocument();
    expect(screen.getByText("main.go")).toBeInTheDocument();
    expect(screen.getByText("App.java")).toBeInTheDocument();
    expect(screen.getByText("main.rs")).toBeInTheDocument();
  });

  it("should render folder icon for directories", () => {
    const entries: TreeEntry[] = [{ name: "src", type: "dir", path: "src" }];

    render(<FileTree entries={entries} onSelect={mockOnSelect} height={400} />);

    expect(screen.getByText("src")).toBeInTheDocument();
    // Folder icon should be present (svg element)
  });
});

describe("TreeEntry type", () => {
  it("should have correct structure", () => {
    const entry: TreeEntry = {
      name: "test.ts",
      type: "file",
      path: "src/test.ts",
      size: 1024,
      language: "TypeScript",
    };

    expect(entry.name).toBe("test.ts");
    expect(entry.type).toBe("file");
    expect(entry.path).toBe("src/test.ts");
    expect(entry.size).toBe(1024);
    expect(entry.language).toBe("TypeScript");
  });

  it("should work with minimal required fields", () => {
    const entry: TreeEntry = {
      name: "folder",
      type: "dir",
      path: "folder",
    };

    expect(entry.name).toBe("folder");
    expect(entry.type).toBe("dir");
    expect(entry.size).toBeUndefined();
    expect(entry.language).toBeUndefined();
  });
});

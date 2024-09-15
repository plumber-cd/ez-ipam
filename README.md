# ez-ipam

This project was inspired by a great tool https://www.davidc.net/sites/default/subnets/subnets.html and its successor https://visualsubnetcalc.com/.

![Demo](./demo.gif)

I only ever had two problems with these tools:

- I needed to be able to leave comments at supernet levels, not just at the last subnet of the tree
- I needed a state management that would not require availability of these websites

Other alternatives like SolarWinds or NetBox or phpIPAM were either not Open Source or just had too many dependencies on things like DB and Redis and required you to run it somewhere.
In my personal oppinion, the best tool is the one that you can run locally.

And also I was in TUI mood lately, and I felt like hacking something up with Go and https://github.com/rivo/tview.

What I ended up with is a TUI that would manage its state in the current working directory in the `.ez-ipam` folder as a bunch of YAML files. Additionally, it would generate a summary to [`EZ-IPAM.md`](./EZ-IPAM.md) in a human readable format, so you only ever need the tool to make changes.

This makes it agnostic to how you want to manage the state, you can put it on a NFS for all I care.
My personal idea was that the state along with the `EZ-IPAM.md` file can be in a Git repository.
That way history of changes is traceable, and Git provides out of the box conflict resolution for when multiple people made conficting changes
(and this is why state in the `.ez-ipam` directory is not a single YAML file, to minimize chances of any conflicts).
As a bonus, if you are using GitHub-like platform, the `EZ-IPAM.md` file can be right there in your browser for when you only want to read it.

This repository itself has a `.ez-ipam` folder with some state in it and a [`EZ-IPAM.md`](./EZ-IPAM.md) file generated from it as a demonstration.
